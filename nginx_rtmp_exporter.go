// MIT License

// Copyright (c) 2022 Mauricio Antunes

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "nginx_rtmp"
)

func newServerMetric(metricName string, docString string, varLabels []string, constLabels prometheus.Labels) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(namespace, "server", metricName), docString, varLabels, constLabels)
}

func newStreamMetric(metricName string, docString string, varLabels []string, constLabels prometheus.Labels) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(namespace, "stream", metricName), docString, varLabels, constLabels)
}

type metrics map[string]*prometheus.Desc
type MediaInfo struct {
	MediaType     string  `json:"media_type"`
	Bitrate       int64   `json:"bitrate"`
	CodecId       int64   `json:"codec_id"`
	ExtraData     string  `json:"extra_data"`
	Width         int64   `json:"width"`
	Height        int64   `json:"height"`
	FrameRate     float64 `json:"frame_rate"`
	CeaCaptions   bool    `json:"cea_captions"`
	Channels      int64   `json:"channels"`
	BitsPerSample int64   `json:"bits_per_sample"`
	SampleRate    int64   `json:"sample_rate"`
}
type Upstream struct {
	ID              string  `json:"id"`
	RemoteAddr      string  `json:"remote_addr"`
	LocalAddr       string  `json:"local_addr"`
	Connection      int64   `json:"connection"`
	Required        bool    `json:"required"`
	ResumeFrom      string  `json:"resume_from"`
	SentBytes       float64 `json:"sent_bytes"`
	Position        int64   `json:"position"`
	AckedFrames     int64   `json:"acked_frames"`
	AckedBytes      float64 `json:"acked_bytes"`
	AutoAckedFrames int64   `json:"auto_acked_frames"`
}
type Upstreams []Upstream
type Video struct {
	KMPOutId      string    `json:"kmp_out_id"`
	InputId       string    `json:"input_id"`
	ChannelId     string    `json:"channel_id"`
	TrackId       string    `json:"track_id"`
	MemLeft       float64   `json:"mem_left"`
	MemLimit      float64   `json:"mem_limit"`
	LastTimestamp int64     `json:"last_timestamp"`
	LastCreated   int64     `json:"last_created"`
	SentFrames    int64     `json:"sent_frames"`
	SentKeyFrames int64     `json:"sent_key_frames"`
	Written       int64     `json:"written"`
	Bitrate       int64     `json:"bitrate"`
	FrameRate     float64   `json:"frame_rate"`
	MediaInfo     MediaInfo `json:"media_info"`
	Upstreams     Upstreams `json:"upstreams"`
}
type Audio struct {
	KMPOutId      string    `json:"kmp_out_id"`
	InputId       string    `json:"input_id"`
	ChannelId     string    `json:"channel_id"`
	TrackId       string    `json:"track_id"`
	MemLeft       float64   `json:"mem_left"`
	MemLimit      float64   `json:"mem_limit"`
	LastTimestamp int64     `json:"last_timestamp"`
	LastCreated   int64     `json:"last_created"`
	SentFrames    int64     `json:"sent_frames"`
	SentKeyFrames int64     `json:"sent_key_frames"`
	Written       int64     `json:"written"`
	Bitrate       int64     `json:"bitrate"`
	FrameRate     float64   `json:"frame_rate"`
	MediaInfo     MediaInfo `json:"media_info"`
	Upstreams     Upstreams `json:"upstreams"`
}
type Tracks struct {
	Video Video `json:"video"`
	Audio Audio `json:"audio"`
}
type Stream struct {
	Name         string  `json:"name"`
	Args         string  `json:"args"`
	Type         string  `json:"type"`
	BWIn         float64 `json:"bw_in"`
	BytesIn      float64 `json:"bytes_in"`
	BWInAudio    float64 `json:"bw_in_audio"`
	BytesInAudio float64 `json:"bytes_in_audio"`
	BWInVideo    float64 `json:"bw_in_video"`
	BytesInVideo float64 `json:"bytes_in_video"`
	BWOut        float64 `json:"bw_out"`
	BytesOut     float64 `json:"bytes_out"`
	Uptime       int64   `json:"uptime"`
	Tracks       Tracks  `json:"tracks"`
}
type Streams []Stream
type Session struct {
	FlashVer   string  `json:"flashver"`
	SWFUrl     string  `json:"swfurl"`
	TCUrl      string  `json:"tc_url"`
	PageUrl    string  `json:"page_url"`
	Type3ExtTs string  `json:"type3_ext_ts"`
	RemoteAddr string  `json:"remote_addr"`
	Uptime     int64   `json:"uptime"`
	Connection int64   `json:"connection"`
	Streams    Streams `json:"streams"`
}
type Sessions []Session
type Application struct {
	Name     string   `json:"name"`
	Sessions Sessions `json:"sessions"`
}
type Applications []Application
type Server struct {
	Applications Applications `json:"applications"`
}
type Servers []Server
type Metric struct {
	Version      string  `json:"version"`
	NginxVersion string  `json:"nginx_version"`
	RTMPVersion  string  `json:"rtmp_version"`
	Compiler     string  `json:"compiler"`
	Built        string  `json:"built"`
	PID          int     `json:"pid"`
	Uptime       int64   `json:"uptime"`
	NAccepted    float64 `json:"naccepted"`
	BWIn         float64 `json:"bw_in"`
	BytesIn      float64 `json:"bytes_in"`
	BWOut        float64 `json:"bw_out"`
	BytesOut     float64 `json:"bytes_out"`
	Severs       Servers `json:"servers"`
}

var (
	serverMetrics = metrics{
		"bytesIn":        newServerMetric("incoming_bytes_total", "Current total of incoming bytes", nil, nil),
		"bytesOut":       newServerMetric("outgoing_bytes_total", "Current total of outgoing bytes", nil, nil),
		"bandwidthIn":    newServerMetric("receive_bytes", "Current bandwidth in per second", nil, nil),
		"bandwidthOut":   newServerMetric("transmit_bytes", "Current bandwidth out per second", nil, nil),
		"currentStreams": newServerMetric("current_streams", "Current number of active streams", nil, nil),
		"uptime":         newServerMetric("uptime_seconds_total", "Number of seconds NGINX-RTMP started", nil, nil),
	}
	streamMetrics = metrics{
		"bytesIn":      newStreamMetric("incoming_bytes_total", "Current total of incoming bytes", []string{"stream"}, nil),
		"bytesOut":     newStreamMetric("outgoing_bytes_total", "Current total of outgoing bytes", []string{"stream"}, nil),
		"bandwidthIn":  newStreamMetric("receive_bytes", "Current bandwidth in per second", []string{"stream"}, nil),
		"bandwidthOut": newStreamMetric("transmit_bytes", "Current bandwidth out per second", []string{"stream"}, nil),
		"uptime":       newStreamMetric("uptime_seconds_total", "Number of seconds since the stream started", []string{"stream"}, nil),
	}
)

// Exporter collects NGINX-RTMP stats from the status page URI
// using the prometheus metrics package
type Exporter struct {
	URI                  string
	mutex                sync.RWMutex
	fetch                func() (io.ReadCloser, error)
	streamNameNormalizer *regexp.Regexp
	logger               log.Logger

	serverMetrics map[string]*prometheus.Desc
	streamMetrics map[string]*prometheus.Desc
}

// ServerInfo characteristics of the RTMP server
type ServerInfo struct {
	BytesIn     float64
	BytesOut    float64
	BandwidthIn float64
	BandwidhOut float64
	Uptime      int64
}

// StreamInfo characteristics of a stream
type StreamInfo struct {
	Name        string
	BytesIn     float64
	BytesOut    float64
	BandwidthIn float64
	BandwidhOut float64
	Uptime      int64
}

// NewServerInfo builds a ServerInfo struct from string values
func NewServerInfo(bytesIn, bytesOut, bandwidthIn, bandwidthOut float64, uptime int64) ServerInfo {
	var bytesInNum, bytesOutNum, bandwidthInNum, bandwidthOutNum float64
	bytesInNum = bytesIn
	bytesOutNum = bytesOut
	bandwidthInNum = bandwidthIn / 1048576   // bandwidth is in bits
	bandwidthOutNum = bandwidthOut / 1048576 // bandwidth is in bits
	return ServerInfo{
		BytesIn:     bytesInNum,
		BytesOut:    bytesOutNum,
		BandwidthIn: bandwidthInNum,
		BandwidhOut: bandwidthOutNum,
		Uptime:      uptime,
	}
}

// NewStreamInfo builds a StreamInfo struct from string values
func NewStreamInfo(name string, bytesIn, bytesOut, bandwidthIn, bandwidthOut float64, uptime int64) StreamInfo {
	var bytesInNum, bytesOutNum, bandwidthInNum, bandwidthOutNum float64
	bytesInNum = bytesIn
	bytesOutNum = bytesOut
	bandwidthInNum = bandwidthIn / 1048576   // bandwidth is in bits
	bandwidthOutNum = bandwidthOut / 1048576 // bandwidth is in bits
	return StreamInfo{
		Name:        name,
		BytesIn:     bytesInNum,
		BytesOut:    bytesOutNum,
		BandwidthIn: bandwidthInNum,
		BandwidhOut: bandwidthOutNum,
		Uptime:      uptime,
	}
}

// NewExporter initializes an exporter
func NewExporter(uri string, timeout time.Duration, streamNameNormalizer *regexp.Regexp, logger log.Logger) (*Exporter, error) {
	return &Exporter{
		URI:                  uri,
		fetch:                fetchStats(uri, timeout),
		streamNameNormalizer: streamNameNormalizer,
		logger:               logger,

		serverMetrics: serverMetrics,
		streamMetrics: streamMetrics,
	}, nil
}

func fetchStats(uri string, timeout time.Duration) func() (io.ReadCloser, error) {
	client := http.Client{
		Timeout: timeout,
	}

	return func() (io.ReadCloser, error) {
		resp, err := client.Get(uri)
		if err != nil {
			return nil, err
		}
		if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
		return resp.Body, nil
	}
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect from concurrent collects
	defer e.mutex.Unlock()

	e.scrape(ch)
}

func parseServerStats(doc Metric) (ServerInfo, error) {
	bytesIn := doc.BytesIn
	bytesOut := doc.BytesOut
	receiveBytes := doc.BWIn
	transmitBytes := doc.BWOut
	uptime := doc.Uptime

	return NewServerInfo(bytesIn, bytesOut, receiveBytes, transmitBytes, uptime), nil
}

func parseStreamsStats(data Metric, streamNameNormalizer *regexp.Regexp) ([]StreamInfo, error) {
	streams := make([]StreamInfo, 0)
	for _, session := range data.Severs[0].Applications[0].Sessions {
		for _, stream := range session.Streams {
			name := streamNameNormalizer.FindString(stream.Name)
			// adding the app name here to ensure that the metrics are unique
			app := ""
			appName := stream.Name
			if len(appName) > 0 {
				app = appName + "-" // dash separator between app and stream names
			}
			bytesIn := stream.BytesIn
			bytesOut := stream.BytesOut
			receiveBytes := stream.BWIn
			transmitBytes := stream.BytesOut
			uptime := stream.Uptime
			streams = append(streams, NewStreamInfo(app+name, bytesIn, bytesOut, receiveBytes, transmitBytes, uptime))
		}
	}
	return streams, nil
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	var doc Metric
	data, err := e.fetch()
	if err != nil {
		level.Error(e.logger).Log("msg", "Can't scrape NGINX-RTMP", "err", err)
		return
	}
	defer data.Close()
	body, err := ioutil.ReadAll(data)
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(body), &doc)
	if err != nil {
		return
	}

	server, err := parseServerStats(doc)
	if err != nil {
		level.Error(e.logger).Log("msg", "Can't parse XML", "err", err)
		return
	}
	ch <- prometheus.MustNewConstMetric(e.serverMetrics["bytesIn"], prometheus.CounterValue, server.BytesIn)
	ch <- prometheus.MustNewConstMetric(e.serverMetrics["bytesOut"], prometheus.CounterValue, server.BytesOut)
	ch <- prometheus.MustNewConstMetric(e.serverMetrics["bandwidthIn"], prometheus.GaugeValue, server.BandwidthIn)
	ch <- prometheus.MustNewConstMetric(e.serverMetrics["bandwidthOut"], prometheus.GaugeValue, server.BandwidhOut)
	ch <- prometheus.MustNewConstMetric(e.serverMetrics["uptime"], prometheus.CounterValue, float64(server.Uptime))

	streams, err := parseStreamsStats(doc, e.streamNameNormalizer)
	if err != nil {
		level.Error(e.logger).Log("msg", "Can't parse XML", "err", err)
		return
	}

	for _, stream := range streams {
		ch <- prometheus.MustNewConstMetric(e.streamMetrics["bytesIn"], prometheus.CounterValue, stream.BytesIn, stream.Name)
		ch <- prometheus.MustNewConstMetric(e.streamMetrics["bytesOut"], prometheus.CounterValue, stream.BytesOut, stream.Name)
		ch <- prometheus.MustNewConstMetric(e.streamMetrics["bandwidthIn"], prometheus.GaugeValue, stream.BandwidthIn, stream.Name)
		ch <- prometheus.MustNewConstMetric(e.streamMetrics["bandwidthOut"], prometheus.GaugeValue, stream.BandwidhOut, stream.Name)
		ch <- prometheus.MustNewConstMetric(e.streamMetrics["uptime"], prometheus.CounterValue, float64(stream.Uptime), stream.Name)
	}

	ch <- prometheus.MustNewConstMetric(e.serverMetrics["currentStreams"], prometheus.GaugeValue, float64(len(streams)))
}

// Describe describes all metrics to be exported to Prometheus
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range e.serverMetrics {
		ch <- metric
	}

	for _, metric := range e.streamMetrics {
		ch <- metric
	}
}

func main() {
	var (
		listenAddress   = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9728").String()
		metricsPath     = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		scrapeURI       = kingpin.Flag("nginxrtmp.scrape-uri", "URI on which to scrape NGINX-RTMP stats.").Default("http://localhost:8001/api/rtmp_kmp/").String()
		timeout         = kingpin.Flag("nginxrtmp.timeout", "Timeout for trying to get stats from NGINX-RTMP.").Default("5s").Duration()
		pidFile         = kingpin.Flag("nginxrtmp.pid-file", "Optional path to a file containing the NGINX-RTMP PID for additional metrics.").Default("").String()
		regexStreamName = kingpin.Flag("nginxrtmp.regex-stream-name", "Regex to normalize stream name from NGINX-RTMP").Default(".*").String()
	)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)
	level.Info(logger).Log("msg", "Starting nginx_rtmp_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "context", version.BuildContext())

	// Compile regex before starting the exporter and exits if it a bad regex
	streamNameNormalizer := regexp.MustCompile(*regexStreamName)

	exporter, err := NewExporter(*scrapeURI, *timeout, streamNameNormalizer, logger)
	if err != nil {
		level.Error(logger).Log("msg", "Error creating an exporter", "err", err)
		os.Exit(1)
	}
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("nginx_rtmp_exporter"))

	level.Info(logger).Log("msg", "PID File:", pidFile)
	if *pidFile != "" {
		procExporter := prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{
			PidFn: func() (int, error) {
				content, err := ioutil.ReadFile(*pidFile)
				if err != nil {
					return 0, fmt.Errorf("cant't read pid file %q: %s", *pidFile, err)
				}
				value, err := strconv.Atoi(strings.TrimSpace(string(content)))
				if err != nil {
					return 0, fmt.Errorf("can't parse pid file %q: %s", *pidFile, err)
				}
				return value, nil
			},
			Namespace: namespace,
		})
		prometheus.MustRegister(procExporter)
	}

	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>NGINX-RTMP exporter</title></head>
			<body>
			<h1>NGINX-RTMP exporter</h1>
			<p><a href='` + *metricsPath + `'>Metrics</a></p>
			</body>
			</html>`,
		))
	})
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
