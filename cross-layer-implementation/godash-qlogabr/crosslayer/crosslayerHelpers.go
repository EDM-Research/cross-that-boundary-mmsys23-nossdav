package crosslayer

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go/qlog"
	"github.com/uccmisl/godash/logging"
)

type AbortLogic int

const (
	Base   AbortLogic = iota
	Rate   AbortLogic = iota
	Double AbortLogic = iota
)

type CrossLayerAccountant struct {
	metricLogger *logging.MetricLogger

	EventChannel   chan qlog.Event
	throughputList []int // list of bytes
	//relativeTimeLastEvent time.Duration
	mu          sync.Mutex
	trackEvents bool

	// Variables used for tracking elapsed time
	totalPassed_ms  int64
	currentlyTiming bool // Indicates if we are currently downloading and tracking time
	currStartTime   time.Time

	// Variables for stall prediction
	predictStall                              bool
	bufferLevel_atStartOfSegment_Milliseconds int
	representationBitrate                     int // kbits / second
	segmentDuration_seconds                   int
	//predictionWindow                          int         // number of packets the predictor looks at
	arrivalTimes                        []time.Time // List of the arrival times of each packet in throughputList
	time_atStartOfSegment               time.Time
	m_cancel                            context.CancelFunc // Is called when the HTTP request needs to be cancelled
	m_aborted                           *bool
	m_maxBuffer_ms                      int
	m_lowestBit_bps                     int
	m_currentSegmentChunksize_bits      int
	m_nextSegmentLowerRepChunksize_bits int
	m_predictionWindowPercentage        float32 // indicates the portion of the current segment that has to be downloaded before an bort can be called
	m_lowerReservoir_ms                 int

	m_abortLogic AbortLogic
}

func (a *CrossLayerAccountant) InitialisePredictor(metricLogger *logging.MetricLogger, abortLogic AbortLogic) {
	fmt.Println("Stall prediction enabled")
	//a.predictionWindow = 20
	a.predictStall = false
	a.metricLogger = metricLogger
	a.m_predictionWindowPercentage = 0.15
	a.m_abortLogic = abortLogic
}

func (a *CrossLayerAccountant) SegmentStart_predictStall(segDuration_s int, repLevel_kbps int, currBufferLevel int, cancel context.CancelFunc, aborted *bool, maxBuffer_ms int, lowestBit_bps int, segmentChunkSize_bits int, nextSegmentLowerReptChunksize_bits int, lowerReservoir_ms int) {
	a.m_currentSegmentChunksize_bits = segmentChunkSize_bits
	a.m_nextSegmentLowerRepChunksize_bits = nextSegmentLowerReptChunksize_bits
	a.predictStall = true
	a.m_cancel = cancel
	a.m_aborted = aborted
	a.m_maxBuffer_ms = maxBuffer_ms
	a.m_lowestBit_bps = lowestBit_bps
	a.StartTiming()
	a.bufferLevel_atStartOfSegment_Milliseconds = currBufferLevel
	a.time_atStartOfSegment = time.Now()
	//fmt.Println("PREDICTORBUFFER: ", a.bufferLevel_atStartOfSegment_Milliseconds)
	a.m_lowerReservoir_ms = lowerReservoir_ms

	// Empty the throughput and timing lists
	a.mu.Lock()
	//fmt.Println("NUMBEROFPACKETS: ", len(a.throughputList))
	a.throughputList = nil
	a.arrivalTimes = nil
	a.mu.Unlock()

	a.segmentDuration_seconds = segDuration_s
	a.representationBitrate = repLevel_kbps
}

func (a *CrossLayerAccountant) SetTrackingEvents(trackEvents bool) {
	a.trackEvents = trackEvents
}

func (a *CrossLayerAccountant) Listen(trackEvents bool) {
	a.totalPassed_ms = 0

	a.SetTrackingEvents(trackEvents)
	go a.channelListenerThread()
}

func (a *CrossLayerAccountant) stallPredictor() {
	a.mu.Lock()
	totalBytes := 0
	for _, el := range a.throughputList {
		totalBytes += el
	}
	sum_bits := totalBytes * 8
	// Time since first packet of this segment
	windowTotalTime_ms := time.Since(a.arrivalTimes[0]).Milliseconds()
	a.mu.Unlock()

	var bitsToDownload int
	var windowBitrate int

	if a.segmentDuration_seconds > 0 && windowTotalTime_ms > 0 {
		bitsToDownload = a.m_currentSegmentChunksize_bits - sum_bits // Number of bytes that need to be downloaded
		// bits / ms  := bits / ms
		windowBitrate = sum_bits / int(windowTotalTime_ms)

		a.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
			TimeStamp: time.Now(),
			Tag:       "WINDOWTHROUGHPUT",
			Message:   strconv.Itoa(windowBitrate),
		}
	}

	a.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
		TimeStamp: time.Now(),
		Tag:       "SUMBITS",
		Message:   strconv.Itoa(sum_bits),
	}

	a.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
		TimeStamp: time.Now(),
		Tag:       "WINDOWTHRESHOLD",
		Message:   fmt.Sprintf("%v", a.m_predictionWindowPercentage*float32(a.m_currentSegmentChunksize_bits)),
	}

	a.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
		TimeStamp: time.Now(),
		Tag:       "SEGMENTCHUNKSIZE",
		Message:   strconv.Itoa(a.m_currentSegmentChunksize_bits),
	}

	// Only do predictions when we have received enough packets
	if float32(sum_bits) > a.m_predictionWindowPercentage*float32(a.m_currentSegmentChunksize_bits) && a.segmentDuration_seconds > 0 {
		//fmt.Println("IN PREDICTION WINDOW")
		/*
				a.mu.Lock()

				// Calculate sum of all bits received
				var sliceOfList []int = a.throughputList[len(a.throughputList)-a.predictionWindow:]
				var sum int = 0
				for _, el := range sliceOfList {
					sum += el
				}
				sum_bits := sum * 8
				predictionWindowStartTime := a.arrivalTimes[len(a.throughputList)-a.predictionWindow]

				// Calculate the average throughput of the prediction window
				windowTotalTime_ms := time.Since(predictionWindowStartTime).Milliseconds()

				a.mu.Unlock()


			// bits 	:=    (kbps == bpms)   		 / ms
			segmentSize := (a.representationBitrate) / a.segmentDuration_Milliseconds
		*/

		// Only do predictions when we have received less bytes than we expect to receive
		if sum_bits < a.m_currentSegmentChunksize_bits && windowTotalTime_ms > 0 {
			// Time it will take in ms to download the remaining bits at this rate
			requiredTime_ms := bitsToDownload / windowBitrate

			// bits 	:=    bps		 / s
			segmentSizeLowestThrough := (a.m_lowestBit_bps) / (a.segmentDuration_seconds)
			requiredTimeLowestThrough_ms := segmentSizeLowestThrough / windowBitrate

			level := a.calculateCurrentBufferLevel()

			a.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
				TimeStamp: time.Now(),
				Tag:       "ABORTLOGIC_REQUIREDTIME",
				Message:   strconv.Itoa(requiredTime_ms),
			}

			a.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
				TimeStamp: time.Now(),
				Tag:       "ABORTLOGIC_LEVEL",
				Message:   strconv.Itoa(level),
			}

			/*a.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
				TimeStamp: time.Now(),
				Tag:       "DEBUGINFO",
				Message:   "RequiredTime_ms " + strconv.Itoa(requiredTime_ms) + " level " + strconv.Itoa(level) + " requiredTimeLowestThrough_ms " + strconv.Itoa(requiredTimeLowestThrough_ms) + " sumbits " + strconv.Itoa(sum_bits) + " currChunk " + strconv.Itoa(a.m_currentSegmentChunksize_bits) + " bitstodownload " + strconv.Itoa(bitsToDownload) + " windowbitrate " + strconv.Itoa(windowBitrate) + " segmentsizelowestthrough " + strconv.Itoa(segmentSizeLowestThrough),
			}*/

			if level <= a.m_lowerReservoir_ms {
				if requiredTime_ms > level && requiredTimeLowestThrough_ms < requiredTime_ms {
					// Report stall prediction
					//fmt.Println("STALLPREDICTOR ", time.Now().UnixMilli())
					a.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
						TimeStamp: time.Now(),
						Tag:       "STALLPREDICTOR",
						Message:   "STALLPREDICTOR",
					}

					*a.m_aborted = true
					a.m_cancel()
				} else {
					if a.m_abortLogic == Double {
						// Calculate if the next segment would be downloaded in time
						requiredTimeNext_ms := a.m_nextSegmentLowerRepChunksize_bits / windowBitrate
						// We predict that the current segment will be downloaded in time, but if the buffer is filled with one segment and we scale one representation downn, will the next segment be downloaded in time at the current rate?
						if requiredTimeNext_ms+requiredTime_ms > level+(a.segmentDuration_seconds*1000) {
							a.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
								TimeStamp: time.Now(),
								Tag:       "STALLPREDICTOR",
								Message:   "STALLPREDICTOR",
							}

							*a.m_aborted = true
							a.m_cancel()
						}
					}
				}
			}
		}
	}
}

func (a *CrossLayerAccountant) calculateCurrentBufferLevel() int {
	passedTime := time.Since(a.time_atStartOfSegment).Milliseconds()
	level := a.bufferLevel_atStartOfSegment_Milliseconds - int(passedTime)

	// Buffer cannot go below 0
	if level < 0 {
		level = 0
	}

	return level
}

func (a *CrossLayerAccountant) channelListenerThread() {
	for msg := range a.EventChannel {
		// Only process events when this bool is set
		if a.trackEvents {
			//a.relativeTimeLastEvent = msg.RelativeTime
			details := msg.GetEventDetails()
			eventType := details.EventType()
			if eventType == "EventPacketReceived" {
				//fmt.Println("CROSSLAYERBUFFERLEVEL", a.calculateCurrentBufferLevel(), time.Now().UnixMilli())
				//fmt.Println(eventType)
				packetReceivedPointer := details.(*qlog.EventPacketReceived)
				//fmt.Println(packetReceivedPointer.Length)
				a.mu.Lock()
				a.throughputList = append(a.throughputList, int(packetReceivedPointer.Length))
				a.mu.Unlock()

				// If we are doing stall predictions, calculate prediction after this packet is received
				if a.predictStall {
					// Measure arrival time as well
					a.mu.Lock()
					a.arrivalTimes = append(a.arrivalTimes, time.Now())
					a.mu.Unlock()

					a.stallPredictor()
				}
			}
		}
	}
}

/**
* Returns average measured throughput in bits/second
 */
func (a *CrossLayerAccountant) GetAverageThroughput() float64 {
	// Calculate sum
	var sum int = 0
	for _, el := range a.throughputList {
		sum += el
	}
	/*
		fmt.Println("Sum XL: ", sum)
			f, err := os.OpenFile("/tmp/trace.csv", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
			if err != nil {
				fmt.Println(err)
			} else {
				f.WriteString(strconv.FormatInt(int64(sum), 10) + "\n")
			}*/
	//     bits			  /  second
	return float64(sum*8) / (float64(a.getTotalTime()) / 1000) // convert it to seconds
}

/**
* Returns average measured throughput in bits/second of last 3000 packets
 */
func (a *CrossLayerAccountant) GetRecentAverageThroughput() float64 {
	// Calculate sum
	var sum int = 0
	if len(a.throughputList) > 3000 {
		var sliceOfList []int = a.throughputList[len(a.throughputList)-3000 : len(a.throughputList)+1]
		for _, el := range sliceOfList {
			sum += el
		}
	} else {
		for _, el := range a.throughputList {
			sum += el
		}
	}
	/*
		fmt.Println("Sum XL: ", sum)
			f, err := os.OpenFile("/tmp/trace.csv", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
			if err != nil {
				fmt.Println(err)
			} else {
				f.WriteString(strconv.FormatInt(int64(sum), 10) + "\n")
			}*/
	//     bits			  /  second
	return float64(sum*8) / (float64(a.getTotalTime()) / 1000) // convert it to seconds
}

// Should be called when we start downloading a segment
func (a *CrossLayerAccountant) StartTiming() {
	a.currStartTime = time.Now()
	a.currentlyTiming = true
}

// Should be called when we have received an entire segment
func (a *CrossLayerAccountant) StopTiming() int {
	a.predictStall = false
	if a.currentlyTiming {
		currPassedTime := time.Since(a.currStartTime)
		currPassedTime_ms := currPassedTime.Milliseconds()

		a.totalPassed_ms += currPassedTime_ms
		a.currentlyTiming = false
		return int(currPassedTime_ms)
	} else {
		fmt.Printf("Warning: stopping timer while timer is not running")
		return 0
	}
}

// Returns the total measured time in miliseconds, even when the current timer is still running
func (a *CrossLayerAccountant) getTotalTime() int64 {
	// If we are currently timing, calculate the current passed time and add it to the total before returning
	if a.currentlyTiming {
		currPassedTime := time.Since(a.currStartTime)
		currPassedTime_ms := currPassedTime.Milliseconds()
		return a.totalPassed_ms + currPassedTime_ms
	} else {
		return a.totalPassed_ms
	}
}
