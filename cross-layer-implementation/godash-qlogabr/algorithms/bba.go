/*
* Arno Verstraete
 */

package algorithms

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/uccmisl/godash/logging"
)

/*
 * Contains all extra information that BBA-2 gathers during all segment decisions
 */
type BBA2Data struct {
	PreviousBufferLevel      int   // In milliseconds
	UsingRate                bool  // Indicates if we are still in startup and need to use the rate-based algorithm
	lowestBitrateChunkList   []int // List of chunk sizes in bits
	previousSegmentNumber    int
	currSegmentNumber        int
	maxAverageChunkRatioList []float32 // Indicates the ratio between the maximum chunk size and the average chunk size, for every representation
	metricLogger             *logging.MetricLogger
}

/*
 * Constructs and initializes a BBA2Data struct
 */
func NewBBA2Data(lowestBitrateChunkListStr string, maxAvgRatioList []float32, logger *logging.MetricLogger) BBA2Data {
	data := BBA2Data{}
	data.PreviousBufferLevel = 0
	data.UsingRate = true
	data.previousSegmentNumber = 1
	data.currSegmentNumber = 1

	data.metricLogger = logger

	max := 0
	sum := 0

	chunkList := strings.Split(lowestBitrateChunkListStr, ",")

	for i := 0; i < len(chunkList); i++ {
		intVal, err := strconv.Atoi(chunkList[i])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		data.lowestBitrateChunkList = append(data.lowestBitrateChunkList, intVal)
	}

	for i := 0; i < len(data.lowestBitrateChunkList); i++ {
		if data.lowestBitrateChunkList[i] > max {
			max = data.lowestBitrateChunkList[i]
		}
		sum += data.lowestBitrateChunkList[i]
	}

	data.maxAverageChunkRatioList = maxAvgRatioList

	return data
}

/*
 * Selects the representation index according to the BBA algorithm
 */
func BBA(bufferLevel_Milliseconds int, maxBufferLevel_Seconds int, highestMPDrepRateIndex int, lowestMPDrepRateIndex int, bandwithList []int,
	segmentDuration_Milliseconds int, debugLog bool, debugFile string, thrList *[]int, newThr int, previousRepRate int) int {

	*thrList = append(*thrList, newThr)

	maxBufferLevel_Milliseconds := maxBufferLevel_Seconds * 1000

	// Static reservoirs
	var reservoir_lower float64 = 0.1 * float64(maxBufferLevel_Milliseconds)
	var reservoir_upper float64 = reservoir_lower

	// If this statement hits there only fits one segment in the reservoir, which could be too little
	if debugLog && segmentDuration_Milliseconds > int(reservoir_lower)/2 {
		logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "The buffer is relatively small for the current segment duration")
	}

	// If we are in the lower reservoir, select the lowest bandwith
	if reservoir_lower >= float64(bufferLevel_Milliseconds) {
		fmt.Println("In lower reservoir")
		return lowestMPDrepRateIndex
	} else if maxBufferLevel_Milliseconds-int(reservoir_upper) <= bufferLevel_Milliseconds {
		// If we are in the higher reservoir, select the highest bandwixth
		fmt.Println("In upper reservoir")
		return highestMPDrepRateIndex
	}

	// Available representation boundaries
	R1 := LowestBitrate(bandwithList)
	Rmax := HighestBitrate(bandwithList)

	// Buffer cushion boundaries
	//B1 := reservoir_lower
	Bm := float64(maxBufferLevel_Milliseconds) - reservoir_lower - reservoir_upper

	// Buffer percentage
	var percentage float64 = (float64(bufferLevel_Milliseconds) / Bm)

	fmt.Println(percentage, bufferLevel_Milliseconds, Bm)

	// Map to a bitrate
	var desiredBitrate float64 = (percentage * Rmax) + R1

	// Choose the representation that best fits this bitrate
	chosenRep := SelectRepRateWithThroughtput(int(desiredBitrate), bandwithList, lowestMPDrepRateIndex)

	// Check if the desired reprate is more than 1 step higher or lower in the rate ladder
	if bandwithList[chosenRep] > previousRepRate {
		chosenRep = previousRepRate - 1
	} else if bandwithList[chosenRep] < previousRepRate {
		chosenRep = previousRepRate + 1
	}

	return chosenRep
}

/*
 *  Calculates the representation index of the next segment according to the BBA-2 algorithm.
 */
func BBA2(bufferLevel_Milliseconds int, maxBufferLevel_Seconds int, highestMPDrepRateIndex int, lowestMPDrepRateIndex int, bandwithList []int,
	segmentDuration_Milliseconds int, debugLog bool, debugFile string, thrList *[]int, newThr int, previousRepRate int, currentSegmentNumber int, data *BBA2Data) int {

	data.previousSegmentNumber = data.currSegmentNumber
	data.currSegmentNumber = currentSegmentNumber

	*thrList = append(*thrList, newThr)

	// Do not do anything if this algorithm is called unnecessarily
	if data.currSegmentNumber == data.previousSegmentNumber {
		// fmt.Println("Skipping")
		return lowestMPDrepRateIndex
	} else if data.currSegmentNumber <= 1 {
		return lowestMPDrepRateIndex
	}

	maxBufferLevel_Milliseconds := maxBufferLevel_Seconds * 1000
	maxBufferLevel_Segments := maxBufferLevel_Milliseconds / segmentDuration_Milliseconds

	// Lower reservoir size is calculated using chunk sizes (in milliseconds)
	var reservoir_lower float64 = float64(calculateBBA2Reservoir(data.lowestBitrateChunkList, maxBufferLevel_Segments*2, currentSegmentNumber, int(LowestBitrate(bandwithList)), segmentDuration_Milliseconds/1000, maxBufferLevel_Milliseconds, data))
	var reservoir_upper float64 = 0.1 * float64(maxBufferLevel_Milliseconds) // We reach Rmax at 90% buffer occupancy

	data.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
		TimeStamp: time.Now(),
		Tag:       "LOWERRESERVOIR",
		Message:   strconv.Itoa(int(reservoir_lower)),
	}

	fmt.Println("RESERVOIR: ", reservoir_lower)

	// If this statement hits there only fits one segment in the reservoir, which could be too little
	if debugLog && segmentDuration_Milliseconds > int(reservoir_lower)/2 {
		logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "The buffer is relatively small for the current segment duration")
	}

	var chosenRep int

	// If we are in the lower reservoir, select the lowest bandwith
	if reservoir_lower >= float64(bufferLevel_Milliseconds) {
		fmt.Println("In lower reservoir")
		chosenRep = lowestMPDrepRateIndex
	} else if maxBufferLevel_Milliseconds-int(reservoir_upper) <= bufferLevel_Milliseconds {
		// If we are in the higher reservoir, select the highest bandwixth
		fmt.Println("In upper reservoir")
		chosenRep = highestMPDrepRateIndex
	} else {
		// Available representation boundaries
		//R1 := LowestBitrate(bandwithList)
		Rmax := HighestBitrate(bandwithList)

		// Buffer cushion boundaries
		//B1 := reservoir_lower
		Bm := float64(maxBufferLevel_Milliseconds) - reservoir_lower - reservoir_upper

		//
		bufferLevel_adjusted_ms := bufferLevel_Milliseconds - int(reservoir_lower)
		// Buffer percentage
		var percentage float64 = (float64(bufferLevel_adjusted_ms) / Bm)
		if percentage < 0 {
			percentage = 0
		}

		fmt.Println(percentage, bufferLevel_Milliseconds, Bm)

		// Map to a bitrate
		var desiredBitrate float64 = (percentage * Rmax)

		data.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
			TimeStamp: time.Now(),
			Tag:       "PERCENTAGE",
			Message:   fmt.Sprintf("%v", percentage),
		}
		data.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
			TimeStamp: time.Now(),
			Tag:       "DESIREDBITRATE",
			Message:   strconv.Itoa(int(desiredBitrate)),
		}

		// Choose the representation that best fits this bitrate
		chosenRep = SelectRepRateWithThroughtput(int(desiredBitrate), bandwithList, lowestMPDrepRateIndex)
	}

	// Check if the desired reprate is more than 1 step higher or lower in the rate ladder
	if bandwithList[chosenRep] > bandwithList[previousRepRate] {
		chosenRep = previousRepRate - 1
		logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Stepping up")
	} else if bandwithList[chosenRep] < bandwithList[previousRepRate] {
		chosenRep = previousRepRate + 1
		logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Stepping down")
	}

	// Use Rate in startup
	if data.UsingRate {
		// Calculate the representation that the Rate baed algorithm would choose
		chosenRep_Rate := rate(segmentDuration_Milliseconds/1000, data, newThr, currentSegmentNumber, bandwithList, previousRepRate, debugFile, debugLog, float64(bufferLevel_Milliseconds)/1000, reservoir_lower/1000, float64(bufferLevel_Milliseconds) < reservoir_upper)

		// We keep using the rate-based algorithm untill BBA2 selects a higher representation than the rate based algorithm, or the buffer drops
		if bandwithList[chosenRep_Rate] >= bandwithList[chosenRep] && data.PreviousBufferLevel <= bufferLevel_Milliseconds {
			chosenRep = chosenRep_Rate
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Using Rate")
		} else if currentSegmentNumber != 0 {
			// If BBA-2 selected a higher rate than Rate, switch to BBA-2 and stop using Rate
			data.UsingRate = false
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Switched from Rate to BBA2")
			data.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
				TimeStamp: time.Now(),
				Tag:       "LOGICSWITCH",
				Message:   "RATE_TO_BBA",
			}
		}
	}

	// Set previous buffer level for next time
	data.PreviousBufferLevel = bufferLevel_Milliseconds

	logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Selected "+strconv.Itoa(chosenRep)+" for segment "+strconv.Itoa(currentSegmentNumber))

	return chosenRep
}

/*
 *  Calculates the reservoir size using chunk sizes
 *  Return Value in milliseconds
 */
func calculateBBA2Reservoir(lowestBitrateChunkList []int, predictionTimePeriod_segments int, currentSegmetNumber int, lowestBitrate_bps int, segmentDuration_seconds int, buffersize_milli int, data *BBA2Data) float32 {
	numberOfSegments := len(lowestBitrateChunkList)
	sum := float32(0)

	// Use currentSeg;entNumber -1, because segments start at 1
	for i := currentSegmetNumber - 1; i < numberOfSegments && i < predictionTimePeriod_segments; i++ {
		// The amount of time it is going to take to download chunk i at a rate of Rmin
		chunkSize := float32(lowestBitrateChunkList[i])
		expectedDownloadTimeChunk := chunkSize / float32(lowestBitrate_bps)
		// The amount of time we gained or lost during the download. We are expectedDownloadTimeChunk seconds busy downloading a segment that will fill the buffer by segmentDuration_seconds.
		bufferDelta := expectedDownloadTimeChunk - float32(segmentDuration_seconds)

		sum += bufferDelta
	}

	sum_milli := sum * 1000

	fmt.Println("BUFFERSIZE: ", buffersize_milli)

	data.metricLogger.WriteChannel <- logging.MetricLoggingFormat{
		TimeStamp: time.Now(),
		Tag:       "CHUNKSUM",
		Message:   strconv.Itoa(int(sum_milli)),
	}

	// Clamp the reservoir between 3 * segmentsize and buffersize
	if sum_milli < float32(3*segmentDuration_seconds*1000) {
		sum_milli = float32(3 * segmentDuration_seconds * 1000)
	}
	if sum_milli > float32(buffersize_milli) {
		sum_milli = float32(buffersize_milli)
	}

	// Return in milliseconds
	return sum_milli
}

func Get_BBA2_LowerReservoir(bufferLevel_Milliseconds int, maxBufferLevel_Seconds int, bandwithList []int, segmentDuration_Milliseconds int, currentSegmentNumber int, data *BBA2Data) int {
	maxBufferLevel_Milliseconds := maxBufferLevel_Seconds * 1000
	maxBufferLevel_Segments := maxBufferLevel_Milliseconds / segmentDuration_Milliseconds
	return int(calculateBBA2Reservoir(data.lowestBitrateChunkList, maxBufferLevel_Segments*2, currentSegmentNumber, int(LowestBitrate(bandwithList)), segmentDuration_Milliseconds/1000, maxBufferLevel_Milliseconds, data))
}

/*
 * Calculates the representation index of the next segment using BBA-2 startup phase rate algorithm
 */
func rate(segmentDuration_seconds int, data *BBA2Data, lastThroughput int, currentSegmentNumber int, bandwithList []int, previousRepRate int,
	debugFile string, debugLog bool, bufferLevel_seconds float64, reservoirSize_seconds float64, underUpperReservoir bool) int {
	// If we are already at the highest representation, stay on the highest representation
	if previousRepRate == 0 && currentSegmentNumber != 1 {
		logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Rate at highest representation")
		return previousRepRate
	}

	var previousSegmentNumber int
	if currentSegmentNumber != 1 {
		previousSegmentNumber = currentSegmentNumber - 1
	} else {
		previousSegmentNumber = 1
	}

	// Select lowest bandwith for the first segment
	if currentSegmentNumber <= 1 {
		logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Rate chooses lowest representation")
		return LowestBitrateIndex(bandwithList)
	}

	// Theoretical increase in buffer after next segment download
	// deltaB = amount of seconds we have downloaded - the time it took to download
	// Use previousSegmentNumber - 1 because segments start at 1
	deltaB := float64(segmentDuration_seconds) - (float64(data.lowestBitrateChunkList[previousSegmentNumber-1]) / float64(lastThroughput))

	logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "DELTAB "+strconv.Itoa(int(deltaB))+" "+strconv.Itoa(int(data.maxAverageChunkRatioList[previousRepRate-1]*float32(bandwithList[previousRepRate-1])*float32(segmentDuration_seconds))))

	// Below is wrong
	// Threshold depens on buffer occupation
	// 0.875 - 0.5
	// 0s    - end of reservoir
	// factor := bufferLevel_seconds / reservoirSize_seconds
	// threshold := 0.875 - (factor * (0.875 - 0.5))
	// if threshold < 0.5 {
	// 	threshold = 0.5
	// }

	// V - (0.5 * V * Y)/e
	// Y = Ri / Ri+1
	Y := float64(bandwithList[previousRepRate]) / float64(bandwithList[previousRepRate-1])
	threshold := float64(segmentDuration_seconds) - ((0.5 * float64(segmentDuration_seconds) * Y) / float64(data.maxAverageChunkRatioList[previousRepRate-1]))
	// TODO maxaverageChunkRatio should be for each representation individually

	fmt.Println("Threshold ", threshold)

	// ARNO V: I beleive the threshold has to lower to      0.5 or 1 - (1/float64(data.maxAverageChunkRatio))
	//         in a linear way
	if !underUpperReservoir {
		threshold = 0.5
		//threshold = 1 - (1/float64(data.maxAverageChunkRatio))
	}

	if deltaB > threshold*float64(segmentDuration_seconds) {
		return previousRepRate - 1
	} else {
		return previousRepRate
	}
}

/*
 * Resets parameters for the BBA algorithm when an abort is called
 */
func ResetBBAData_afterAbort(data *BBA2Data, bufferLevel_milli int) {
	data.PreviousBufferLevel = bufferLevel_milli
	data.UsingRate = true
}
