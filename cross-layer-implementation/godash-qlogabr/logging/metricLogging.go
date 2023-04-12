package logging

import (
	"os"
	"strconv"
	"time"

	glob "github.com/uccmisl/godash/global"
)

type MetricLoggingFormat struct {
	TimeStamp time.Time
	Tag       string
	Message   string
}

type MetricLogger struct {
	startTimeUnix         int64
	WriteChannel          chan MetricLoggingFormat
	bufferLevelMilli      int
	lastBufferLevelUpdate time.Time
	pollFrequencyMilli    int
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func (a *MetricLogger) StartLogger(pollFrequencyMilli int, bandwithList []int, bufferSize int) {
	// Initialise logger
	a.pollFrequencyMilli = pollFrequencyMilli
	a.WriteChannel = make(chan MetricLoggingFormat)
	startTime := time.Now()
	a.startTimeUnix = startTime.UnixMilli()

	a.SetBufferLevel(0)

	// Write logs non-blocking
	go a.WriteLog()
	go a.MetricsPoller()

	a.WriteChannel <- MetricLoggingFormat{
		TimeStamp: time.Now(),
		Tag:       "HIGHESTBANDWIDTH",
		Message:   strconv.Itoa(bandwithList[0]),
	}

	a.WriteChannel <- MetricLoggingFormat{
		TimeStamp: time.Now(),
		Tag:       "BUFFERSIZE",
		Message:   strconv.Itoa(bufferSize),
	}

	a.WriteChannel <- MetricLoggingFormat{
		TimeStamp: time.Now(),
		Tag:       "STARTTIME",
		Message:   strconv.Itoa(int(a.startTimeUnix)),
	}
}

func (a *MetricLogger) WriteLog() {
	// Open log file
	f, err := os.OpenFile(glob.MetricsLogLoctation, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	check(err)

	// Close the file when the logger ends
	defer f.Close()

	// Keep popping messages from the channel
	for log := range a.WriteChannel {
		f.WriteString(strconv.Itoa(int(log.TimeStamp.UnixMilli()-a.startTimeUnix)) + " " + log.Tag + " " + log.Message + "\n")
	}
}

func (a *MetricLogger) SetBufferLevel(buffLevelMilli int) {
	a.bufferLevelMilli = buffLevelMilli
	a.lastBufferLevelUpdate = time.Now()

	// Log bufferlevel
	/*a.WriteChannel <- MetricLoggingFormat{
		TimeStamp: a.lastBufferLevelUpdate,
		Tag:       "BUFFERLEVEL",
		Message:   strconv.Itoa(buffLevelMilli),
	}*/
}

func (a *MetricLogger) CalculateCurrentBufferOccupancy() int {
	diffMilli := time.Since(a.lastBufferLevelUpdate).Milliseconds()
	bufferLevelMilli := a.bufferLevelMilli - int(diffMilli)
	if bufferLevelMilli < 0 {
		bufferLevelMilli = 0
	}
	return bufferLevelMilli
}

func (a *MetricLogger) MetricsPoller() {
	for true {
		// Log bufferlevel
		a.WriteChannel <- MetricLoggingFormat{
			TimeStamp: time.Now(),
			Tag:       "BUFFERLEVEL",
			Message:   strconv.Itoa(a.CalculateCurrentBufferOccupancy()),
		}

		// Sleep for the given duration
		time.Sleep(time.Duration(a.pollFrequencyMilli) * time.Millisecond)
	}
}
