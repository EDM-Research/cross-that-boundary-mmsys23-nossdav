/*
 *	goDASH, golang client emulator for DASH video streaming
 *	Copyright (c) 2019, Jason Quinlan, Darijo Raca, University College Cork
 *											[j.quinlan,d.raca]@cs.ucc.ie)
 *                      Maëlle Manifacier, MISL Summer of Code 2019, UCC
 *	This program is free software; you can redistribute it and/or
 *	modify it under the terms of the GNU General Public License
 *	as published by the Free Software Foundation; either version 2
 *	of the License, or (at your option) any later version.
 *
 *	This program is distributed in the hope that it will be useful,
 *	but WITHOUT ANY WARRANTY; without even the implied warranty of
 *	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *	GNU General Public License for more details.
 *
 *	You should have received a copy of the GNU General Public License
 *	along with this program; if not, write to the Free Software
 *	Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA
 *	02110-1301, USA.
 */

// ./goDASH -url "[http://cs1dev.ucc.ie/misl/4K_non_copyright_dataset/10_sec/x264/bbb/DASH_Files/full/bbb_enc_x264_dash.mpd, http://cs1dev.ucc.ie/misl/4K_non_copyright_dataset/10_sec/x264/bbb/DASH_Files/full/bbb_enc_x264_dash.mpd]" -adapt default -codec AVC -debug true -initBuffer 2 -maxBuffer 10 -maxHeight 1080 -numSegments 20  -storeDASH 347985

package player

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/uccmisl/godash/P2Pconsul"
	"github.com/uccmisl/godash/algorithms"
	algo "github.com/uccmisl/godash/algorithms"
	"github.com/uccmisl/godash/crosslayer"
	glob "github.com/uccmisl/godash/global"
	"github.com/uccmisl/godash/hlsfunc"
	"github.com/uccmisl/godash/http"
	"github.com/uccmisl/godash/logging"
	"github.com/uccmisl/godash/qoe"
	"github.com/uccmisl/godash/utils"

	xlayer "github.com/uccmisl/godash/crosslayer"
	abrqlog "github.com/uccmisl/godash/qlog"
)

// play position
var playPosition = 0

// current segment number
var segmentNumber = 1
var segmentDuration int
var nextSegmentNumber int

// current buffer level
var bufferLevel = 0
var maxBufferLevel int
var waitToPlayCounter = 0
var stallTime = 0

// current mpd file
var mpdListIndex = 0
var lowestMPDrepRateIndex []int
var highestMPDrepRateIndex []int

// save the previous mpdIndex
var oldMPDIndex = 0

// determine if an MPD is byte-range or not
var isByteRangeMPD bool
var startRange = 0
var endRange = 0

// current representation rate
var repRate = 0

//var repRatesReversed bool

// current adaptationSet
var currentMPDRepAdaptSet int

// Segment size (in bits)
var segSize int

// baseURL for this MPD file
var baseURL string
var headerURL string
var currentURL string

// we need to keep a tab on the different size segments - use this for now
// we will use an array in the future
var segmentDurationTotal = 0
var segmentDurationArray []int

// the list of bandwith values (rep_rates) from the current MPD file
var bandwithList []int

// list of throughtputs - noted from downloading the segments
var thrList []int

// time values
var startTime time.Time
var nextRunTime time.Time
var arrivalTime int

// additional output logs values
var repCodec string
var repHeight int
var repWidth int
var repFps int
var mimeType string

// used to calculate targetRate - float64
var kP = 0.01
var kI = 0.001
var staticAlgParameter = 0.0

// first step is to check the first MPD for the codec (I had problem passing a
// 2-dimensional array, so I moved the check to here)
var codecList [][]string
var codecIndexList [][]int
var usedVideoCodec bool
var codecIndex int
var audioContent bool
var onlyAudio bool
var audioRate int
var audioCodec string

var urlInput []string

// For the mapSegments of segments :
// Map with the segment number and a structure of informations
// one map contains all content
var mapSegmentLogPrintouts []map[int]logging.SegPrintLogInformation

// a map of maps containing segment header information
var segHeadValues map[int]map[int][]int

// default value for the exponential ratio
var exponentialRatio float64

// file download location
var fileDownloadLocation string

// printHeadersData local
var printHeadersData map[string]string

// print the log to terminal
var printLog bool

// variable to determine if we are using the goDASHbed testbed
var useTestbedBool bool

// variable to determine if we should generate QoE values
var getQoEBool bool

// variable to determine if we should save our streaming files
var saveFilesBool bool

// other QoE variables
var segRates []float64
var sumSegRate float64
var totalStallDur float64
var nStalls int
var nSwitches int
var rateChange []float64
var sumRateChange float64
var rateDifference float64

// index values for the types of MPD types
var mimeTypes []int
var mimeTypesMediaType []abrqlog.MediaType

var streamStructs []http.StreamStruct
var BBA2DataStruct algo.BBA2Data

// Stream :
/*
 * get the header file for the current video clip
 * check the different arguments in order to stream
 * call streamLoop to begin to stream
 */
func Stream(mpdList []http.MPD, debugFile string, debugLog bool, codec string, codecName string, maxHeight int, streamDuration int, streamSpeed float64, maxBuffer int, initBuffer int, adapt string, urlString string, fileDownloadLocationIn string, extendPrintLog bool, hls string, hlsBool bool, quic string, quicBool bool, getHeaderBool bool, getHeaderReadFromFile string, exponentialRatioIn float64, printHeadersDataIn map[string]string, printLogIn bool,
	useTestbedBoolIn bool, getQoEBoolIn bool, saveFilesBoolIn bool, Noden P2Pconsul.NodeUrl, accountant *xlayer.CrossLayerAccountant) {

	// set debug logs for the collab clients
	if Noden.ClientName != glob.CollabPrintOff && Noden.ClientName != "" {
		Noden.SetDebug(debugFile, debugLog)
	}

	// check if the codec is in the MPD urls passed in
	codecList, codecIndexList, audioContent = http.GetCodec(mpdList, codec, debugLog)
	// determine if the passed in codec is one of the codecs we use (checking the first MPD only)
	// fmt.Println(codecList)
	// fmt.Println(codecIndexList)
	// fmt.Println(audioContent)
	usedVideoCodec, codecIndex = utils.FindInStringArray(codecList[0], codec)

	// logs
	var mapSegmentLogPrintouts []map[int]logging.SegPrintLogInformation

	// set local val
	exponentialRatio = exponentialRatioIn
	fileDownloadLocation = fileDownloadLocationIn
	printHeadersData = printHeadersDataIn
	printLog = printLogIn
	useTestbedBool = useTestbedBoolIn
	getQoEBool = getQoEBoolIn
	saveFilesBool = saveFilesBoolIn

	// check the codec and print error is false
	// if !usedVideoCodec {
	// 	// print error message
	// 	fmt.Printf("*** -" + codecName + " " + codec + " is not in the first provided MPD, please check " + urlString + " ***\n")
	// 	// stop the app
	// 	utils.StopApp()
	// }
	if codecList[0][0] == glob.RepRateCodecAudio && len(codecList[0]) == 1 {
		logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "*** This is an audio only file, ignoring Video Codec - "+codec+" ***\n")
		onlyAudio = true
		// reset the codeIndex to suit Audio only
		codecIndex = 0
		//codecIndexList[0][codecIndex] = 0
	} else if !usedVideoCodec {
		// print error message
		logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "*** -"+glob.CodecName+" "+codec+" is not in the provided MPD, please check "+urlString+" ***\n")
		// stop the app
		utils.StopApp()
	}

	// Start the metrics logger
	var metricsLogger logging.MetricLogger

	// the input must be a defined value - loops over the adaptationSets
	// currently one adaptation set per video and audio
	for currentMPDRepAdaptSetIndex := range codecIndexList[mpdListIndex] {

		// only use the selected input codec and audio (if audio exists)
		if codecIndexList[0][currentMPDRepAdaptSetIndex] != -1 {

			currentMPDRepAdaptSet = currentMPDRepAdaptSetIndex

			// lets work out how many mimeTypes we have
			mimeTypes = append(mimeTypes, currentMPDRepAdaptSetIndex)

			//TODO better mimetypeparsing
			mimeTypeString := http.GetRepresentationMimeType(mpdList[mpdListIndex], currentMPDRepAdaptSetIndex)
			currentMediaType := abrqlog.MediaTypeOther
			if strings.Contains(mimeTypeString, "video") {
				currentMediaType = abrqlog.MediaTypeVideo
			} else if strings.Contains(mimeTypeString, "audio") {
				currentMediaType = abrqlog.MediaTypeAudio
			} else if strings.Contains(mimeTypeString, "text") {
				currentMediaType = abrqlog.MediaTypeSubtitles
			}
			mimeTypesMediaType = append(mimeTypesMediaType, currentMediaType)

			// currentMPDRepAdaptSet = 1
			// determine if we are using a byte-range or standard MPD profile
			// the xml Representation>BaseURL is saved in the same location
			// for byte range full, main and onDemand
			// so check for BaseURL, if not empty, then its a byte-range
			baseURL = http.GetRepresentationBaseURL(mpdList[mpdListIndex], 0)
			if baseURL != glob.RepRateBaseURL {
				isByteRangeMPD = true
				logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Byte-range MPD: ")
			}

			// get the relevent values from this MPD
			// maxSegments - number of segments to download
			// maxBufferLevel - maximum buffer level in seconds
			// highestMPDrepRateIndex - index with the highest rep_rate
			// lowestMPDrepRateIndex - index with the lowest rep_rate
			// segmentDuration - segment duration
			// bandwithList - get all the range of representation bandwiths of the MPD

			// maxSegments was the first value
			l_highestMPDrepRateIndex := 0
			l_lowestMPDrepRateIndex := 0
			_, maxBufferLevel, l_highestMPDrepRateIndex, l_lowestMPDrepRateIndex, segmentDurationArray, bandwithList, baseURL = http.GetMPDValues(mpdList, mpdListIndex, maxHeight, streamDuration, maxBuffer, currentMPDRepAdaptSet, isByteRangeMPD, debugLog)

			highestMPDrepRateIndex = append(highestMPDrepRateIndex, l_highestMPDrepRateIndex)
			lowestMPDrepRateIndex = append(lowestMPDrepRateIndex, l_lowestMPDrepRateIndex)

			// get the profile for this file
			profiles := strings.Split(mpdList[mpdListIndex].Profiles, ":")
			numProfile := len(profiles) - 2
			profile := profiles[numProfile]

			// if byte-range add this to the file name
			if isByteRangeMPD {
				profile += glob.ByteRangeString
			}

			logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "DASH profile for the header is: "+profile)

			// reset repRate
			repRate = l_lowestMPDrepRateIndex

			// print values to debug log
			logging.DebugPrint(debugFile, debugLog, "\nDEBUG: ", "streaming has begun")
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Input values to streaming algorithm: "+adapt)
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "maxHeight: "+strconv.Itoa(maxHeight))
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "streamDuration in seconds: "+strconv.Itoa(streamDuration))
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "streamSpeed: "+fmt.Sprintf("%f", streamSpeed))
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "maxBuffer: "+strconv.Itoa(maxBuffer))
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "initBuffer: "+strconv.Itoa(initBuffer))
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "url: "+urlString)
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "fileDownloadLocation: "+fileDownloadLocation)
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "HLS: "+hls)
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "extend: "+strconv.FormatBool(extendPrintLog))

			// update audio rate and codec
			AudioByteRange := false
			if audioContent && codecList[0][currentMPDRepAdaptSetIndex] == glob.RepRateCodecAudio {
				audioRate = mpdList[mpdListIndex].Periods[0].AdaptationSet[len(mimeTypes)-1].Representation[repRate].BandWidth / 1000
				audioCodec = mpdList[mpdListIndex].Periods[0].AdaptationSet[len(mimeTypes)-1].Representation[repRate].Codecs
				if isByteRangeMPD {
					AudioByteRange = true
					logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Audio Byte-Range Header")
				}
			}

			// get the stream header from the required MPD (first index in the mpdList)
			headerURL = http.GetFullStreamHeader(mpdList[mpdListIndex], isByteRangeMPD, currentMPDRepAdaptSet, AudioByteRange, 0)
			headerURL = strings.Replace(headerURL, "$Bandwidth$", strconv.Itoa(mpdList[mpdListIndex].Periods[0].AdaptationSet[currentMPDRepAdaptSet].Representation[l_lowestMPDrepRateIndex].BandWidth), -1)
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "stream initialise URL header: "+headerURL)

			// convert the url strings to a list
			urlInput = http.URLList(urlString)

			// get the current url - trim any white space
			currentURL = strings.TrimSpace(urlInput[mpdListIndex])
			// currentURL := strings.TrimSpace(urlInput[mpdListIndex])
			logging.DebugPrint(debugFile, debugLog, "\nDEBUG: ", "current URL header: "+currentURL)

			// set the segmentDuration to the first passed in URL
			segmentDuration = segmentDurationArray[0]

			// Collaborative Code - Start
			OriginalURL := currentURL
			OriginalbaseURL := baseURL
			baseJoined := baseURL + headerURL
			urlHeaderString := http.JoinURL(currentURL, baseURL+headerURL, debugLog)
			if Noden.ClientName != glob.CollabPrintOff && Noden.ClientName != "" {
				currentURL = Noden.Search(urlHeaderString, segmentDuration, true, profile)

				logging.DebugPrint(debugFile, debugLog, "\nDEBUG: ", "current URL joined: "+currentURL)
				currentURL = strings.Split(currentURL, "::")[0]
				logging.DebugPrint(debugFile, debugLog, "\nDEBUG: ", "current URL joined: "+currentURL)
				urlSplit := strings.Split(currentURL, "/")
				logging.DebugPrint(debugFile, debugLog, "\nDEBUG: ", "current URL joined: "+urlSplit[len(urlSplit)-1])
				baseJoined = urlSplit[len(urlSplit)-1]
			}
			// Collaborative Code - End

			ctx2 := context.Background()

			bba2Based := false

			// determine the inital variables to set, based on the algorithm choice
			switch adapt {
			case glob.ConventionalAlg:
				// there is no byte range in this file, so we set byte-range bool to false
				// we don't want to add the seg duration to this file, so 'addSegDuration' is false
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)
				// set the inital rep_rate to the lowest value index
				repRate = l_lowestMPDrepRateIndex
			case glob.ElasticAlg:
				//fmt.Println("Elastic / in player.go")
				//fmt.Println("currentURL: ", currentURL)
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)
				repRate = l_lowestMPDrepRateIndex
				///fmt.Println("MPD file repRate index: ", repRate)
				//fmt.Println("MPD file bandwithList[repRate]", bandwithList[repRate])
			case glob.ProgressiveAlg:
				// get the header file
				// there is no byte range in this file, so we set byte-range bool to false
				http.GetFileProgressively(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber, segmentDuration, false, debugLog, AudioByteRange, profile)
			case glob.TestAlg:
				fmt.Println("testAlg / in player.go")
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, AudioByteRange, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)

				//fmt.Println("lowestmpd: ", lowestMPDrepRateIndex)
				repRate = l_lowestMPDrepRateIndex

			case glob.BBAAlg:
				//fmt.Println("BBAAlg / in player.go")
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)

				repRate = l_lowestMPDrepRateIndex

			case glob.ArbiterAlg:
				//fmt.Println("ArbiterAlg / in player.go")
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)

				repRate = l_lowestMPDrepRateIndex

			case glob.LogisticAlg:
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)
				repRate = l_lowestMPDrepRateIndex
			case glob.MeanAverageAlg:
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)
			case glob.GeomAverageAlg:
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)
			case glob.EMWAAverageAlg:
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)
			case glob.MeanAverageXLAlg:
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)
			case glob.MeanAverageRecentXLAlg:
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)
			case glob.BBA1Alg_AV:
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)

				repRate = l_lowestMPDrepRateIndex
			case glob.BBA1Alg_AVXL:
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)

				repRate = l_lowestMPDrepRateIndex
			case glob.BBA2Alg_AV:
				bba2Based = true
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)

				repRate = l_lowestMPDrepRateIndex
			case glob.BBA2Alg_AVXL_base:
				bba2Based = true
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)

				repRate = l_lowestMPDrepRateIndex
			case glob.BBA2Alg_AVXL_rate:
				bba2Based = true
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)

				repRate = l_lowestMPDrepRateIndex
			case glob.BBA2Alg_AVXL_double:
				bba2Based = true
				http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
					segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, currentMediaType, ctx2)

				repRate = l_lowestMPDrepRateIndex
			}

			if bba2Based {
				var chunksLowest string = ""
				var chunksLowestBandwidth int
				var maxAvgRatioList []float32
				for i := 0; i < len(mpdList[mpdListIndex].Periods); i++ {
					for j := 0; j < len(mpdList[mpdListIndex].Periods[i].AdaptationSet); j++ {
						for k := 0; k < len(mpdList[mpdListIndex].Periods[i].AdaptationSet[j].Representation); k++ {
							if mpdList[mpdListIndex].Periods[i].AdaptationSet[j].Representation[k].Chunks != "" {
								if chunksLowest == "" {
									chunksLowest = mpdList[mpdListIndex].Periods[i].AdaptationSet[j].Representation[k].Chunks
									chunksLowestBandwidth = mpdList[mpdListIndex].Periods[i].AdaptationSet[j].Representation[k].BandWidth
								} else if chunksLowestBandwidth > mpdList[mpdListIndex].Periods[i].AdaptationSet[j].Representation[k].BandWidth {
									chunksLowest = mpdList[mpdListIndex].Periods[i].AdaptationSet[j].Representation[k].Chunks
									chunksLowestBandwidth = mpdList[mpdListIndex].Periods[i].AdaptationSet[j].Representation[k].BandWidth
								}
							}
							maxAvgRatioList = append(maxAvgRatioList, mpdList[mpdListIndex].Periods[i].AdaptationSet[j].Representation[k].MaxAvgRatio)
						}
					}
				}
				BBA2DataStruct = algo.NewBBA2Data(chunksLowest, maxAvgRatioList, &metricsLogger)
			}

			// debug logs
			logging.DebugPrint(debugFile, debugLog, "\nDEBUG: ", "We are using repRate: "+strconv.Itoa(repRate))
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "We are using : "+adapt+" for streaming")

			//create the map for the print log
			var mapSegmentLogPrintout map[int]logging.SegPrintLogInformation
			mapSegmentLogPrintout = make(map[int]logging.SegPrintLogInformation)

			//StartTime of downloading
			startTime = time.Now()
			nextRunTime = time.Now()
			//fmt.Println("STARTTIME_GODASH ", startTime.UnixMilli())

			_, client, _ := http.GetHTTPClient(quicBool, glob.DebugFile, debugLog, useTestbedBool)

			// get the segment headers and stop this run
			if getHeaderBool {
				// get the segment headers for all MPD url passed as arguments - print to file
				http.GetAllSegmentHeaders(mpdList, codecIndexList, maxHeight, 1, streamDuration, isByteRangeMPD, maxBuffer, headerURL, codec, urlInput, debugLog, true, client)

				// print error message
				fmt.Printf("*** - All segment header have been downloaded to " + glob.DebugFolder + " - ***\n")
				// exit the application
				os.Exit(3)
			} else {
				if getHeaderReadFromFile == glob.GetHeaderOnline {
					// get the segment headers for all MPD url passed as arguments - not from file
					segHeadValues = http.GetAllSegmentHeaders(mpdList, codecIndexList, maxHeight, 1, streamDuration, isByteRangeMPD, maxBuffer, headerURL, codec, urlInput, debugLog, false, client)
				} else if getHeaderReadFromFile == glob.GetHeaderOffline {
					// get the segment headers for all MPD url passed as arguments - yes from file
					// get headers from file for a given number of seconds of stream time
					// let's assume every n seconds
					segHeadValues = http.GetNSegmentHeaders(mpdList, codecIndexList, maxHeight, 1, streamDuration, isByteRangeMPD, maxBuffer, headerURL, codec, urlInput, debugLog, true, client)

				}
			}

			// I need to have two of more sets of lists for the following content
			streaminfo := http.StreamStruct{
				SegmentNumber:         segmentNumber,
				CurrentURL:            OriginalURL,
				InitBuffer:            initBuffer,
				MaxBuffer:             maxBuffer,
				CodecName:             codecName,
				Codec:                 codec,
				UrlString:             urlString,
				UrlInput:              urlInput,
				MpdList:               mpdList,
				Adapt:                 adapt,
				MaxHeight:             maxHeight,
				IsByteRangeMPD:        isByteRangeMPD,
				StartTime:             startTime,
				NextRunTime:           nextRunTime,
				ArrivalTime:           arrivalTime,
				OldMPDIndex:           0,
				NextSegmentNumber:     0,
				Hls:                   hls,
				HlsBool:               hlsBool,
				MapSegmentLogPrintout: mapSegmentLogPrintout,
				StreamDuration:        streamDuration,
				StreamSpeed:           streamSpeed,
				ExtendPrintLog:        extendPrintLog,
				HlsUsed:               false,
				BufferLevel:           bufferLevel,
				SegmentDurationTotal:  segmentDurationTotal,
				Quic:                  quic,
				QuicBool:              quicBool,
				BaseURL:               OriginalbaseURL,
				DebugLog:              debugLog,
				AudioContent:          audioContent,
				RepRate:               repRate,
				BandwithList:          bandwithList,
				Profile:               profile,
			}
			streamStructs = append(streamStructs, streaminfo)
			mapSegmentLogPrintouts = append(mapSegmentLogPrintouts, mapSegmentLogPrintout)
		}
	}

	// reset currentMPDRepAdaptSet
	// currentMPDRepAdaptSet = 0

	// print the output log headers
	logging.PrintHeaders(extendPrintLog, fileDownloadLocation, glob.LogDownload, debugFile, debugLog, printLog, printHeadersData)

	metricsLogger.StartLogger(100, bandwithList, maxBuffer)

	if adapt == glob.BBA1Alg_AVXL || adapt == glob.BBA2Alg_AVXL_base {
		accountant.InitialisePredictor(&metricsLogger, crosslayer.Base)
	} else if adapt == glob.BBA2Alg_AVXL_rate {
		accountant.InitialisePredictor(&metricsLogger, crosslayer.Rate)
	} else if adapt == glob.BBA2Alg_AVXL_double {
		accountant.InitialisePredictor(&metricsLogger, crosslayer.Double)
	}

	// Streaming loop function - using the first MPD index - 0, and hlsUsed false
	segmentNumber, mapSegmentLogPrintouts = streamLoop(streamStructs, Noden, accountant, &metricsLogger)

	// print sections of the map to the debug log - if debug is true
	if debugLog {
		logging.PrintsegInformationLogMap(debugFile, debugLog, mapSegmentLogPrintouts[0])
	}

	// print out the rest of the play out segments - based on playStartPosition of the last segment streamed
	// and an end time that includes for the original initial buffer size in seconds
	logging.PrintPlayOutLog(mapSegmentLogPrintouts[0][segmentNumber-1].PlayStartPosition+mapSegmentLogPrintouts[0][initBuffer].PlayStartPosition, initBuffer, mapSegmentLogPrintouts, glob.LogDownload, printLog, printHeadersData)

	time.Sleep(1 * time.Second)
	abrqlog.MainTracer.Close()
}

var currently_playing = false

// streamLoop :
/*
 * take the first segment number, download it with a low quality
 * call itself with the next segment number
 */
func streamLoop(streamStructs []http.StreamStruct, Noden P2Pconsul.NodeUrl, accountant *xlayer.CrossLayerAccountant, metricsLogger *logging.MetricLogger) (int, []map[int]logging.SegPrintLogInformation) {

	// variable for rtt for this segment
	var rtt time.Duration
	// has this chunk been replaced by hls
	var hlsReplaced = "no"
	// if we undertake HLS, we need to revise the buffer values
	var bufferDifference int
	// if we set this chunk to HLS used
	if streamStructs[0].HlsUsed {
		hlsReplaced = "yes"
	}
	var segURL string

	// save point for the HTTP protocol used
	var protocol string

	//
	var segmentFileName string

	//
	var P1203Header float64

	// logging info
	// var mapSegmentLogPrintouts []map[int]logging.SegPrintLogInformation

	// lets loop over our mimeTypes
	for mimeTypeIndex := range mimeTypes {

		// get the values from the stream struct
		segmentNumber := streamStructs[mimeTypeIndex].SegmentNumber
		currentURL := streamStructs[mimeTypeIndex].CurrentURL
		initBuffer := streamStructs[mimeTypeIndex].InitBuffer
		maxBuffer := streamStructs[mimeTypeIndex].MaxBuffer
		codecName := streamStructs[mimeTypeIndex].CodecName
		codec := streamStructs[mimeTypeIndex].Codec
		urlString := streamStructs[mimeTypeIndex].UrlString
		urlInput := streamStructs[mimeTypeIndex].UrlInput
		mpdList := streamStructs[mimeTypeIndex].MpdList
		adapt := streamStructs[mimeTypeIndex].Adapt
		maxHeight := streamStructs[mimeTypeIndex].MaxHeight
		isByteRangeMPD := streamStructs[mimeTypeIndex].IsByteRangeMPD
		startTime := streamStructs[mimeTypeIndex].StartTime
		nextRunTime := streamStructs[mimeTypeIndex].NextRunTime
		arrivalTime := streamStructs[mimeTypeIndex].ArrivalTime
		oldMPDIndex := streamStructs[mimeTypeIndex].OldMPDIndex
		nextSegmentNumber := streamStructs[mimeTypeIndex].NextSegmentNumber
		hls := streamStructs[mimeTypeIndex].Hls
		hlsBool := streamStructs[mimeTypeIndex].HlsBool
		mapSegmentLogPrintout := streamStructs[mimeTypeIndex].MapSegmentLogPrintout
		streamDuration := streamStructs[mimeTypeIndex].StreamDuration
		streamSpeed := streamStructs[mimeTypeIndex].StreamSpeed
		extendPrintLog := streamStructs[mimeTypeIndex].ExtendPrintLog
		hlsUsed := streamStructs[mimeTypeIndex].HlsUsed
		bufferLevel := streamStructs[mimeTypeIndex].BufferLevel
		segmentDurationTotal := streamStructs[mimeTypeIndex].SegmentDurationTotal
		quic := streamStructs[mimeTypeIndex].Quic
		quicBool := streamStructs[mimeTypeIndex].QuicBool
		baseURL := streamStructs[mimeTypeIndex].BaseURL
		debugLog := streamStructs[mimeTypeIndex].DebugLog
		audioContent := streamStructs[mimeTypeIndex].AudioContent
		repRate := streamStructs[mimeTypeIndex].RepRate
		bandwithList := streamStructs[mimeTypeIndex].BandwithList
		profile := streamStructs[mimeTypeIndex].Profile

		// determine the MimeType and mimeTypeIndex - set video by default
		// get the mimeType of this adaptationSet
		mimeType = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypeIndex].Representation[repRate].MimeType

		// update audio rate and codec
		AudioByteRange := false
		if audioContent && mimeType == glob.RepRateCodecAudio {
			audioRate = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].BandWidth / 1000
			audioCodec = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Codecs
			if isByteRangeMPD {
				AudioByteRange = true
				logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "Audio Byte-Range Segment")
			}
		}

		logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current MimeType header: "+mimeType)
		/*
		 * Function  :
		 * let's think about HLS - chunk replacement
		 * before we decide what chunks to change, lets create a file for HLS
		 * then add functions to switch out an old chunk
		 */
		// only use HLS if we have at least one segment to replacement
		if hlsBool && segmentNumber > 1 &&
			mimeType == glob.RepRateCodecVideo {
			switch hls {
			// passive - least amount of replacement
			case glob.HlsOn:
				if segmentNumber == 6 {
					// hlsUsed is set to true
					chunkReplace := 5
					var thisRunTimeVal int
					// replace a previously downloaded segment with this call
					nextSegmentNumber, mapSegmentLogPrintouts, bufferDifference, thisRunTimeVal, nextRunTime =
						hlsfunc.GetHlsSegment(
							streamLoop,
							chunkReplace,
							mapSegmentLogPrintouts,
							maxHeight,
							urlInput,
							initBuffer,
							maxBuffer,
							codecName,
							codec,
							urlString,
							mpdList,
							nextSegmentNumber,
							extendPrintLog,
							startTime,
							nextRunTime,
							arrivalTime,
							true,
							quic,
							quicBool,
							baseURL,
							glob.DebugFile,
							debugLog,
							glob.RepRateBaseURL,
							audioContent,
							repRate,
							mimeTypeIndex,
							Noden,
							accountant,
							metricsLogger,
						)

					// change the current buffer to reflect the time taken to get this HLS segment
					bufferLevel -= (int(float64(thisRunTimeVal)*streamSpeed) + bufferDifference)

					// change the buffer levels of the previous chunks, so the printout reflects this value
					mapSegmentLogPrintouts[mimeTypeIndex] = hlsfunc.ChangeBufferLevels(mapSegmentLogPrintouts[mimeTypeIndex], segmentNumber, chunkReplace, bufferDifference)
				}
			}
		}

		// if we have changed the MPD, we need to update some variables
		if oldMPDIndex != mpdListIndex {

			// set the new mpdListIndex
			mpdListIndex = oldMPDIndex

			// get the current url - trim any white space
			currentURL = strings.TrimSpace(urlInput[mpdListIndex])
			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current URL header: "+currentURL)

			// get the relavent values from this MPD
			l_highestMPDrepRateIndex := 0
			l_lowestMPDrepRateIndex := 0
			streamDuration, maxBufferLevel, l_highestMPDrepRateIndex, l_lowestMPDrepRateIndex, segmentDurationArray, bandwithList, baseURL = http.GetMPDValues(mpdList, mpdListIndex, maxHeight, streamDuration, maxBuffer, mimeTypes[mimeTypeIndex], isByteRangeMPD, debugLog)
			highestMPDrepRateIndex[mimeTypeIndex] = l_highestMPDrepRateIndex
			lowestMPDrepRateIndex[mimeTypeIndex] = l_lowestMPDrepRateIndex

			// current segment duration
			segmentDuration = segmentDurationArray[mpdListIndex]

			// ONLY CHANGE THE NUMBER OF SEGMENTS HERE
			//	numSegments := streamDuration / segmentDuration

			//	fmt.Println(segmentNumber)
			//	fmt.Println(segmentDuration)
			//	fmt.Println(numSegments)

			// determine if the passed in codec is one of the codecs we use (checking the current MPD)
			usedVideoCodec, codecIndex = utils.FindInStringArray(codecList[mpdListIndex], codec)
			// check the codec and print error is false
			// if !usedVideoCodec {
			// 	// print error message
			// 	fmt.Printf("*** -" + codecName + " " + codec + " is not in the provided MPD, please check " + urlString + " ***\n")
			// 	// stop the app
			// 	utils.StopApp()
			// }
			if codecList[0][0] == glob.RepRateCodecAudio && len(codecList[0]) == 1 {
				logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "*** This is an audio only file, ignoring Video Codec - "+codec+" ***\n")
				onlyAudio = true
				// reset the codeIndex to suit Audio only
				codecIndex = 0
				//codecIndexList[0][codecIndex] = 0
			} else if !usedVideoCodec {
				// print error message
				logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "*** -"+glob.CodecName+" "+codec+" is not in the provided MPD, please check "+urlString+" ***\n")
				// stop the app
				utils.StopApp()
			}

			// save the current MPD Rep_rate Adaptation Set
			mimeTypes[mimeTypeIndex] = codecIndexList[mpdListIndex][codecIndex]

			// get the profile for this file
			profiles := strings.Split(mpdList[mpdListIndex].Profiles, ":")
			numProfile := len(profiles) - 2
			profile = profiles[numProfile]

			// if byte-range add this to the file name
			if isByteRangeMPD {
				profile += glob.ByteRangeString
			}
		}
		logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "DASH profile for this segment is: "+profile)

		// break out if we have downloaded all of our segments
		// which is current segment duration total plus the next segment to be downloaded
		if segmentDurationTotal+(segmentDuration*glob.Conversion1000) > streamDuration &&
			mimeTypeIndex == len(mimeTypes)-1 {
			// save the current log
			streamStructs[mimeTypeIndex].MapSegmentLogPrintout = mapSegmentLogPrintout
			// get the logs for all adaptationSets
			for mimeTypeIndex := range mimeTypes {
				mapSegmentLogPrintouts = append(mapSegmentLogPrintouts, streamStructs[mimeTypeIndex].MapSegmentLogPrintout)
			}

			playhead := abrqlog.NewPlayheadStatus()
			playhead.PlayheadTime = time.Duration(playPosition) * time.Millisecond
			abrqlog.MainTracer.EndStream(playhead)

			return segmentNumber, mapSegmentLogPrintouts
		}

		// keep rep_rate within the index boundaries
		// MISL - might cause problems
		if repRate < highestMPDrepRateIndex[mimeTypeIndex] {
			logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "Changing rep_rate index: from "+strconv.Itoa(repRate)+" to "+strconv.Itoa(highestMPDrepRateIndex[mimeTypeIndex]))
			repRate = highestMPDrepRateIndex[mimeTypeIndex]
		}

		// get the segment
		if isByteRangeMPD {
			segURL, startRange, endRange = http.GetNextByteRangeURL(mpdList[mpdListIndex], segmentNumber, repRate, mimeTypes[mimeTypeIndex])
			logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "byte start range: "+strconv.Itoa(startRange))
			logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "byte end range: "+strconv.Itoa(endRange))
		} else {
			segURL = http.GetNextSegment(mpdList[mpdListIndex], segmentNumber, repRate, mimeTypes[mimeTypeIndex])
		}
		logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "current segment URL: "+segURL)

		// Collaborative Code - Start
		OriginalURL := currentURL
		OriginalBaseURL := baseURL
		baseJoined := baseURL + segURL
		urlHeaderString := http.JoinURL(currentURL, baseURL+segURL, debugLog)
		if Noden.ClientName != glob.CollabPrintOff && Noden.ClientName != "" {
			currentURL = Noden.Search(urlHeaderString, segmentDuration, true, profile)

			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current URL joined: "+currentURL)
			currentURL = strings.Split(currentURL, "::")[0]
			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current URL joined: "+currentURL)
			urlSplit := strings.Split(currentURL, "/")
			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current URL joined: "+urlSplit[len(urlSplit)-1])
			baseJoined = urlSplit[len(urlSplit)-1]
		}
		// Collaborative Code - End

		ctx, cancel := context.WithCancel(context.Background())
		aborted := false

		// Start Time of this segment
		currentTime := time.Now()
		// The abort logic needs the chunk size of the next segment of one representation lower for predictions
		nextSegmentLowerReprateChunkSize := utils.GetChunk(mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Chunks, segmentNumber+1)
		if repRate > 0 {
			nextSegmentLowerReprateChunkSize = utils.GetChunk(mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate-1].Chunks, segmentNumber+1)
		}
		switch adapt {
		case glob.MeanAverageXLAlg:
			accountant.StartTiming()
		case glob.MeanAverageRecentXLAlg:
			accountant.StartTiming()
		case glob.BBA1Alg_AV:
			accountant.StartTiming()
		case glob.BBA1Alg_AVXL:
			if repRate != lowestMPDrepRateIndex[mimeTypeIndex] {
				accountant.SegmentStart_predictStall(segmentDuration, bandwithList[repRate], bufferLevel, cancel, &aborted, maxBuffer*glob.Conversion1000, bandwithList[utils.GetLowestRepRateIndex(bandwithList)], utils.GetChunk(mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Chunks, segmentNumber), nextSegmentLowerReprateChunkSize, algo.Get_BBA2_LowerReservoir(bufferLevel, maxBuffer, bandwithList, segmentDuration*1000, segmentNumber, &BBA2DataStruct))
			}
		case glob.BBA2Alg_AV:
			accountant.StartTiming()
		case glob.BBA2Alg_AVXL_base:
			if repRate != lowestMPDrepRateIndex[mimeTypeIndex] {
				accountant.SegmentStart_predictStall(segmentDuration, bandwithList[repRate], bufferLevel, cancel, &aborted, maxBuffer*glob.Conversion1000, bandwithList[utils.GetLowestRepRateIndex(bandwithList)], utils.GetChunk(mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Chunks, segmentNumber), nextSegmentLowerReprateChunkSize, algo.Get_BBA2_LowerReservoir(bufferLevel, maxBuffer, bandwithList, segmentDuration*1000, segmentNumber, &BBA2DataStruct))
			}
		case glob.BBA2Alg_AVXL_rate:
			if repRate != lowestMPDrepRateIndex[mimeTypeIndex] {
				accountant.SegmentStart_predictStall(segmentDuration, bandwithList[repRate], bufferLevel, cancel, &aborted, maxBuffer*glob.Conversion1000, bandwithList[utils.GetLowestRepRateIndex(bandwithList)], utils.GetChunk(mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Chunks, segmentNumber), nextSegmentLowerReprateChunkSize, algo.Get_BBA2_LowerReservoir(bufferLevel, maxBuffer, bandwithList, segmentDuration*1000, segmentNumber, &BBA2DataStruct))
			}
		case glob.BBA2Alg_AVXL_double:
			if repRate != lowestMPDrepRateIndex[mimeTypeIndex] {
				accountant.SegmentStart_predictStall(segmentDuration, bandwithList[repRate], bufferLevel, cancel, &aborted, maxBuffer*glob.Conversion1000, bandwithList[utils.GetLowestRepRateIndex(bandwithList)], utils.GetChunk(mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Chunks, segmentNumber), nextSegmentLowerReprateChunkSize, algo.Get_BBA2_LowerReservoir(bufferLevel, maxBuffer, bandwithList, segmentDuration*1000, segmentNumber, &BBA2DataStruct))
			}
		}

		metricsLogger.SetBufferLevel(bufferLevel)
		metricsLogger.WriteChannel <- logging.MetricLoggingFormat{
			TimeStamp: time.Now(),
			Tag:       "SegmentDownloadStart",
			Message:   strconv.Itoa(bandwithList[repRate]),
		}

		var status int
		//fmt.Println("CURRSEGMENTNUMBER", segmentNumber)

		//fmt.Println("GETTINGSEGMENT", time.Now().UnixMilli())

		// Download the segment - add the segment duration to the file name
		switch adapt {
		case glob.ConventionalAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.ElasticAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.ProgressiveAlg:
			rtt, segSize = http.GetFileProgressively(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, debugLog, AudioByteRange, profile)
		case glob.LogisticAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.MeanAverageAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.GeomAverageAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.EMWAAverageAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.TestAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.ArbiterAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.BBAAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.MeanAverageXLAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.MeanAverageRecentXLAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.BBA1Alg_AV:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.BBA1Alg_AVXL:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.BBA2Alg_AV:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.BBA2Alg_AVXL_base:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.BBA2Alg_AVXL_rate:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		case glob.BBA2Alg_AVXL_double:
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctx)
		}

		//fmt.Println("segSize: ", segSize)

		// arrival and delivery times for this segment
		arrivalTime = int(time.Since(startTime).Nanoseconds() / (glob.Conversion1000 * glob.Conversion1000))
		deliveryTime := int(time.Since(currentTime).Nanoseconds() / (glob.Conversion1000 * glob.Conversion1000)) //Time in milliseconds
		thisRunTimeVal := int(time.Since(nextRunTime).Nanoseconds() / (glob.Conversion1000 * glob.Conversion1000))
		prevNextRunTime := nextRunTime
		nextRunTime = time.Now()

		//fmt.Println("deliveryTime: ", deliveryTime)
		accountant.StopTiming()

		//fmt.Println(status, aborted)
		logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", strconv.Itoa(status))
		metricsLogger.WriteChannel <- logging.MetricLoggingFormat{
			TimeStamp: time.Now(),
			Tag:       "SegmentArrived",
			Message:   strconv.Itoa(bandwithList[repRate]),
		}

		if aborted {
			// Reset BBA2 to startup parameters
			if adapt == glob.BBA2Alg_AV || adapt == glob.BBA2Alg_AVXL_base || adapt == glob.BBA2Alg_AVXL_rate || adapt == glob.BBA2Alg_AVXL_double {
				algorithms.ResetBBAData_afterAbort(&BBA2DataStruct, bufferLevel)
			}
			//fmt.Println("After sleep")
			//time.Sleep(8 * time.Second)
			///fmt.Println("After sleep")
			// We will not restart abort detection because we do not want to abort again
			repRate = utils.GetLowestRepRateIndex(bandwithList)

			// keep rep_rate within the index boundaries
			// MISL - might cause problems
			/*if repRate < highestMPDrepRateIndex[mimeTypeIndex] {
				logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "Changing rep_rate index: from "+strconv.Itoa(repRate)+" to "+strconv.Itoa(highestMPDrepRateIndex[mimeTypeIndex]))
				repRate = highestMPDrepRateIndex[mimeTypeIndex]
			}*/

			metricsLogger.WriteChannel <- logging.MetricLoggingFormat{
				TimeStamp: time.Now(),
				Tag:       "SegmentDownloadStart",
				Message:   strconv.Itoa(bandwithList[repRate]),
			}

			// get the segment
			if isByteRangeMPD {
				segURL, startRange, endRange = http.GetNextByteRangeURL(mpdList[mpdListIndex], segmentNumber, repRate, mimeTypes[mimeTypeIndex])
				logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "byte start range: "+strconv.Itoa(startRange))
				logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "byte end range: "+strconv.Itoa(endRange))
			} else {
				segURL = http.GetNextSegment(mpdList[mpdListIndex], segmentNumber, repRate, mimeTypes[mimeTypeIndex])
			}
			logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "current segment URL: "+segURL)

			// Collaborative Code - Start
			OriginalURL = currentURL
			OriginalBaseURL = baseURL
			baseJoined := baseURL + segURL
			urlHeaderString := http.JoinURL(currentURL, baseURL+segURL, debugLog)
			if Noden.ClientName != glob.CollabPrintOff && Noden.ClientName != "" {
				currentURL = Noden.Search(urlHeaderString, segmentDuration, true, profile)

				logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current URL joined: "+currentURL)
				currentURL = strings.Split(currentURL, "::")[0]
				logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current URL joined: "+currentURL)
				urlSplit := strings.Split(currentURL, "/")
				logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current URL joined: "+urlSplit[len(urlSplit)-1])
				baseJoined = urlSplit[len(urlSplit)-1]
			}

			ctxaborted := context.Background()

			// Start Time of this segment
			//fmt.Println("GETTINGSEGMENT", time.Now().UnixMilli())
			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "ABORT has choosen rep_Rate "+strconv.Itoa(repRate)+" @ a rate of "+strconv.Itoa(bandwithList[repRate]/glob.Conversion1000))
			currentTime = time.Now()
			rtt, segSize, protocol, segmentFileName, P1203Header, status = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile, mimeTypesMediaType[mimeTypeIndex], ctxaborted)
			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "Abort segment arrived")
			//intln("SEGMENTARRIVED", bandwithList[repRate], time.Now().UnixMilli())
			// arrival and delivery times for this segment
			arrivalTime = int(time.Since(startTime).Nanoseconds() / (glob.Conversion1000 * glob.Conversion1000))
			deliveryTime = int(time.Since(currentTime).Nanoseconds() / (glob.Conversion1000 * glob.Conversion1000)) //Time in milliseconds
			thisRunTimeVal = int(time.Since(prevNextRunTime).Nanoseconds() / (glob.Conversion1000 * glob.Conversion1000))

			nextRunTime = time.Now()

			metricsLogger.WriteChannel <- logging.MetricLoggingFormat{
				TimeStamp: time.Now(),
				Tag:       "SegmentArrived",
				Message:   strconv.Itoa(bandwithList[repRate]),
			}
		} else {
			//fmt.Println("SEGMENTARRIVED", bandwithList[repRate], time.Now().UnixMilli())
		}

		// some times we want to wait for an initial number of segments before stream begins
		// no need to do asny printouts when we are replacing this chunk
		// && !hlsReplaced
		if initBuffer <= waitToPlayCounter || currently_playing {

			if !currently_playing {
				currently_playing = true
				playhead := abrqlog.NewPlayheadStatus()
				playhead.PlayheadTime = 0
				playhead.PlayheadFrame = 0
				abrqlog.MainTracer.PlayerInteraction(abrqlog.InteractionStatePlay, playhead, streamSpeed)
			}

			// get the segment less the initial buffer
			// this needs to be based on running time and not based on number segments
			// I'll need a function for this
			//playoutSegmentNumber := segmentNumber - initBuffer

			// only print this out if we are not hls replaced
			if !hlsUsed {
				// print out the content of the segment that is currently passed to the player
				var printLogs []map[int]logging.SegPrintLogInformation
				printLogs = append(printLogs, mapSegmentLogPrintout)
				logging.PrintPlayOutLog(arrivalTime, initBuffer, printLogs, glob.LogDownload, printLog, printHeadersData)
			}

			// get the current buffer (excluding the current segment)
			currentBuffer := (bufferLevel - int(float64(thisRunTimeVal)*streamSpeed))

			// if we have a buffer level then we have no stalls
			if currentBuffer >= 0 {
				stallTime = 0

				// if the buffer is empty, then we need to calculate
			} else {
				stallTime = currentBuffer

				playhead := abrqlog.NewPlayheadStatus()
				playhead.PlayheadTime = time.Duration(playPosition) * time.Millisecond
				abrqlog.MainTracer.Rebuffer(playhead)

				bufferStats := abrqlog.NewBufferStats()
				bufferStats.PlayoutTime = time.Duration(0)
				bufferStats.MaxTime = time.Duration(streamStructs[mimeTypeIndex].MaxBuffer) * time.Second
				abrqlog.MainTracer.UpdateBufferOccupancy(mimeTypesMediaType[mimeTypeIndex],
					bufferStats)
			}

			// To have the bufferLevel we take the max between the remaining buffer and 0, we add the duration of the segment we downloaded
			bufferLevel = utils.Max(bufferLevel-int(float64(thisRunTimeVal)*streamSpeed), 0) + (segmentDuration * glob.Conversion1000)

			// increment the waitToPlayCounter
			waitToPlayCounter++

		} else {
			// If we reach this it means that the buffer has once reached the initial desired level, after this we never want to wait for it to fill up again before we start playing
			//inStartupPhase = false
			// add to the current buffer before we start to play
			bufferLevel += (segmentDuration * glob.Conversion1000)
			// increment the waitToPlayCounter
			waitToPlayCounter++
		}

		// check if the buffer level is higher than the max buffer
		if bufferLevel > maxBuffer*glob.Conversion1000 {
			// retrieve the time it is going to sleep from the buffer level
			// sleep until the max buffer level is reached
			sleepTime := int(float64(bufferLevel-(maxBuffer*glob.Conversion1000)) / streamSpeed)
			// sleep
			time.Sleep(time.Duration(sleepTime) * time.Millisecond)

			// reset the buffer to the new value less sleep time - should equal maxBuffer
			bufferLevel -= int(float64(sleepTime) * streamSpeed)
		}

		// some times we want to wait for an initial number of segments before stream begins
		// if we are going to print out some additonal log headers, then get these values
		if extendPrintLog && initBuffer < waitToPlayCounter {
			// base the play out position on the buffer level
			playPosition = segmentDurationTotal + (segmentDuration * glob.Conversion1000) - bufferLevel
			// we need to keep a tab on the different size segments - use this for now
			segmentDurationTotal += (segmentDuration * glob.Conversion1000)
		} else {
			segmentDurationTotal += (segmentDuration * glob.Conversion1000)
		}

		// if we are going to print out some additonal log headers, then get these values
		if extendPrintLog {

			// get the current codec
			repCodec = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Codecs

			// change the codec into something we can understand
			// switch {
			// case strings.Contains(repCodec, "avc"):
			// 	// set the inital rep_rate to the lowest value
			// 	repCodec = glob.RepRateCodecAVC
			// case strings.Contains(repCodec, "hev"):
			// 	repCodec = glob.RepRateCodecHEVC
			// case strings.Contains(repCodec, "vp"):
			// 	repCodec = glob.RepRateCodecVP9
			// case strings.Contains(repCodec, "av1"):
			// 	repCodec = glob.RepRateCodecAV1
			// }

			switch {
			case strings.Contains(repCodec, "avc"):
				repCodec = glob.RepRateCodecAVC
			case strings.Contains(repCodec, "hev"):
				repCodec = glob.RepRateCodecHEVC
			case strings.Contains(repCodec, "hvc1"):
				repCodec = glob.RepRateCodecHEVC
			case strings.Contains(repCodec, "vp"):
				repCodec = glob.RepRateCodecVP9
			case strings.Contains(repCodec, "av1"):
				repCodec = glob.RepRateCodecAV1
			case strings.Contains(repCodec, "mp4a"):
				repCodec = glob.RepRateCodecAudio
			case strings.Contains(repCodec, "ac-3"):
				repCodec = glob.RepRateCodecAudio
			}

			// get rep_rate height, width and frames per second
			repHeight = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Height
			repWidth = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Width
			repFps = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].FrameRate
		}

		// calculate the throughtput (we get the segSize while downloading the file)
		// multiple segSize by 8 to get bits and not bytes
		thr := algo.CalculateThroughtput(segSize*8, deliveryTime)
		//fmt.Println("THROUGHPUT: ", strconv.Itoa(thr))

		// save the bitrate from the input segment (less the header info)
		var kbps float64
		if getQoEBool {
			if val, ok := printHeadersData[glob.P1203Header]; ok {
				if val == "on" || val == "On" {

					// we use this to read from a file
					// kbps = qoe.GetKBPS(segmentFileName, int64(segmentDuration), debugLog, isByteRangeMPD, segSize)

					// we do this to read from our buffer values
					kbps = P1203Header
				}
			}
			// lets move the logic setup for the QoE values from the algorithms to player
			// we don't need to save the segRate as this is also called 'Bandwidth'
			// segRate := float64(log[j].Bandwidth)

			// add this to the seg rate slice
			if segmentNumber > 1 {
				// append to the segRates list
				segRates = append(mapSegmentLogPrintout[segmentNumber-1].SegmentRates, float64(bandwithList[repRate]))
				// sum the seg rates
				sumSegRate = mapSegmentLogPrintout[segmentNumber-1].SumSegRate + float64(bandwithList[repRate])
				// sum the total stall duration
				totalStallDur = float64(mapSegmentLogPrintout[segmentNumber-1].StallTime) + float64(stallTime)
				// get the number of stalls
				if stallTime > 0 {
					// increment the number of stalls
					nStalls = mapSegmentLogPrintout[segmentNumber-1].NumStalls + 1
				} else {
					// otherwise save the number of stalls from the previous log
					nStalls = mapSegmentLogPrintout[segmentNumber-1].NumStalls
				}
				// get the number of switches
				if bandwithList[repRate] == mapSegmentLogPrintout[segmentNumber-1].Bandwidth {
					// store the previous value of switches
					nSwitches = mapSegmentLogPrintout[segmentNumber-1].NumSwitches
				} else {
					// increment the number of switches
					nSwitches = mapSegmentLogPrintout[segmentNumber-1].NumSwitches + 1
				}
				rateDifference = math.Abs(float64(bandwithList[repRate]) - float64(mapSegmentLogPrintout[segmentNumber-1].Bandwidth))
				sumRateChange = mapSegmentLogPrintout[segmentNumber-1].SumRateChange + rateDifference
				rateChange = append(mapSegmentLogPrintout[segmentNumber-1].RateChange, rateDifference)

			} else {

				// otherwise create the list
				segRates = append(segRates, float64(bandwithList[repRate]))
				// sum the seg rates
				sumSegRate = float64(bandwithList[repRate])
				// sum the total stall duration
				totalStallDur = float64(stallTime)
				// get the number of stalls
				if stallTime > 0 {
					// increment the number of stalls
					nStalls = 1
				} else {
					// otherwise set to zero (may not be needed, go might default to zero)
					nStalls = 0
				}
				// get the number of switches
				nSwitches = 0
			}
		}

		// Print to output log
		//printLog(strconv.Itoa(segmentNumber), strconv.Itoa(arrivalTime), strconv.Itoa(deliveryTime), strconv.Itoa(Abs(stallTime)), strconv.Itoa(bandwithList[repRate]/1000), strconv.Itoa((segSize*8)/deliveryTime), strconv.Itoa((segSize*8)/(segmentDuration*1000)), strconv.Itoa(segSize), strconv.Itoa(bufferLevel), adapt, strconv.Itoa(segmentDuration*1000), extendPrintLog, repCodec, strconv.Itoa(repWidth), strconv.Itoa(repHeight), strconv.Itoa(repFps), strconv.Itoa(playPosition), strconv.FormatFloat(float64(rtt.Nanoseconds())/1000000, 'f', 3, 64), fileDownloadLocation)

		// store the current segment log output information in a map
		printInformation := logging.SegPrintLogInformation{
			ArrivalTime:          arrivalTime,
			DeliveryTime:         deliveryTime,
			StallTime:            stallTime,
			Bandwidth:            bandwithList[repRate],
			DelRate:              thr,
			ActRate:              (segSize * 8) / (segmentDuration * glob.Conversion1000),
			SegSize:              segSize,
			P1203HeaderSize:      P1203Header,
			BufferLevel:          bufferLevel,
			Adapt:                adapt,
			SegmentDuration:      segmentDuration,
			ExtendPrintLog:       extendPrintLog,
			RepCodec:             repCodec,
			RepWidth:             repWidth,
			RepHeight:            repHeight,
			RepFps:               repFps,
			PlayStartPosition:    segmentDurationTotal,
			PlaybackTime:         playPosition,
			Rtt:                  float64(rtt.Nanoseconds()) / (glob.Conversion1000 * glob.Conversion1000),
			FileDownloadLocation: fileDownloadLocation,
			RepIndex:             repRate,
			MpdIndex:             mpdListIndex,
			AdaptIndex:           mimeTypes[mimeTypeIndex],
			SegmentIndex:         nextSegmentNumber,
			SegReplace:           hlsReplaced,
			Played:               false,
			HTTPprotocol:         protocol,
			P1203Kbps:            kbps,
			SegmentFileName:      segmentFileName,
			SegmentRates:         segRates,
			SumSegRate:           sumSegRate,
			TotalStallDur:        totalStallDur,
			NumStalls:            nStalls,
			NumSwitches:          nSwitches,
			RateDifference:       rateDifference,
			SumRateChange:        sumRateChange,
			RateChange:           rateChange,
			MimeType:             mimeType,
			Profile:              profile,
		}

		// this saves per segment number so from 1 on, and not 0 on
		// remember this :)
		mapSegmentLogPrintout[segmentNumber] = printInformation

		// if we want to create QoE, then pass in the printInformation and save the QoE values to log
		// don't save json when using collaborative
		var saveCollabFilesBool bool
		if Noden.ClientName != glob.CollabPrintOff && Noden.ClientName != "" {
			saveCollabFilesBool = false
		} else {
			saveCollabFilesBool = saveFilesBool
		}
		if getQoEBool {
			qoe.CreateQoE(&mapSegmentLogPrintout, debugLog, initBuffer, bandwithList[highestMPDrepRateIndex[mimeTypeIndex]], printHeadersData, saveCollabFilesBool, audioRate, audioCodec)
		}

		preRepRate := repRate

		//fmt.Println("BUFFERLEVEL: ", bufferLevel)

		// to calculate throughtput and select the repRate from it (in algorithm.go)
		switch adapt {
		//Conventional Algo
		case glob.ConventionalAlg:
			//fmt.Println("old: ", repRate)
			algo.Conventional(&thrList, thr, &repRate, bandwithList, lowestMPDrepRateIndex[mimeTypeIndex])
			//fmt.Println("new: ", repRate)
			//Harmonic Mean Algo
		case glob.ElasticAlg:
			//fmt.Println("old repRate index: ", repRate)
			//fmt.Println("old bandwithList[repRate]", bandwithList[repRate])
			algo.ElasticAlgo(&thrList, thr, deliveryTime, maxBuffer, &repRate, bandwithList, &staticAlgParameter, bufferLevel, kP, kI, lowestMPDrepRateIndex[mimeTypeIndex])
			//fmt.Println("new repRate index: ", repRate)
			//fmt.Println("new bandwithList[repRate]", bandwithList[repRate])
			//fmt.Println("elastic segmentNumber: ", segmentNumber)
			//fmt.Println("segURL: ", segURL)
		//Progressive Algo
		case glob.ProgressiveAlg:
			// fmt.Println("old: ", repRate)
			algo.Conventional(&thrList, thr, &repRate, bandwithList, lowestMPDrepRateIndex[mimeTypeIndex])
			// fmt.Println("new: ", repRate)
		//Logistic Algo
		case glob.LogisticAlg:
			// fmt.Println("old: ", repRate)
			algo.Logistic(&thrList, thr, &repRate, bandwithList, bufferLevel,
				highestMPDrepRateIndex[mimeTypeIndex], lowestMPDrepRateIndex[mimeTypeIndex], glob.DebugFile, debugLog,
				maxBufferLevel)
			// fmt.Println("new: ", repRate)
			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "reprate returned: "+strconv.Itoa(repRate))
		//Mean Average Algo
		case glob.MeanAverageAlg:
			//fmt.Println("old: ", repRate)
			algo.MeanAverageAlgo(&thrList, thr, &repRate, bandwithList, lowestMPDrepRateIndex[mimeTypeIndex])
			//fmt.Println("new: ", repRate)
		//Geometric Average Algo
		case glob.GeomAverageAlg:
			//fmt.Println("old: ", repRate)
			algo.GeomAverageAlgo(&thrList, thr, &repRate, bandwithList, lowestMPDrepRateIndex[mimeTypeIndex])
			//fmt.Println("new: ", repRate)
		//Exponential Average Algo
		case glob.EMWAAverageAlg:
			//fmt.Println("old: ", repRate)
			algo.EMWAAverageAlgo(&thrList, &repRate, exponentialRatio, 3, thr, bandwithList, lowestMPDrepRateIndex[mimeTypeIndex])

		case glob.ArbiterAlg:

			repRate = algo.CalculateSelectedIndexArbiter(thr, segmentDuration*1000, segmentNumber, maxBufferLevel,
				repRate, &thrList, streamDuration, mpdList[mpdListIndex], currentURL,
				mimeTypes[mimeTypeIndex], segmentNumber, baseURL, debugLog, deliveryTime, bufferLevel,
				highestMPDrepRateIndex[mimeTypeIndex], lowestMPDrepRateIndex[mimeTypeIndex], bandwithList,
				segSize, quicBool, useTestbedBool)
			//fmt.Println("new: ", repRate)
		case glob.BBAAlg:
			//fmt.Println("segDur: ", segmentDuration*1000)
			//fmt.Println("index rate: ", repRate)
			//fmt.Println("baseURL: ", baseURL)
			//fmt.Println("downloadDurationLastSegment: ", deliveryTime)
			//fmt.Println("maxStreamDuration: ", streamDuration)
			//fmt.Println("bufferLevel: ", bufferLevel)
			//fmt.Println("")

			repRate = algo.CalculateSelectedIndexBba(thr, segmentDuration*1000, segmentNumber, maxBufferLevel,
				repRate, &thrList, streamDuration, mpdList[mpdListIndex], currentURL,
				mimeTypes[mimeTypeIndex], segmentNumber, baseURL, debugLog, deliveryTime, bufferLevel,
				highestMPDrepRateIndex[mimeTypeIndex], lowestMPDrepRateIndex[mimeTypeIndex], bandwithList, quicBool, useTestbedBool)

		case glob.TestAlg:
			//fmt.Println("")

		case glob.MeanAverageXLAlg:
			//fmt.Println("old: ", repRate)
			algo.MeanAverageXLAlgo(accountant, &thrList, thr, &repRate, bandwithList, lowestMPDrepRateIndex[mimeTypeIndex])
		case glob.MeanAverageRecentXLAlg:
			//fmt.Println("old: ", repRate)
			algo.MeanAverageRecentXLAlgo(accountant, &thrList, thr, &repRate, bandwithList, lowestMPDrepRateIndex[mimeTypeIndex])
		case glob.BBA1Alg_AV:
			repRate = algo.BBA(bufferLevel, maxBufferLevel, highestMPDrepRateIndex[mimeTypeIndex], lowestMPDrepRateIndex[mimeTypeIndex], bandwithList, segmentDuration*1000, debugLog, glob.DebugFile, &thrList, thr, preRepRate)
		case glob.BBA1Alg_AVXL:
			repRate = algo.BBA(bufferLevel, maxBufferLevel, highestMPDrepRateIndex[mimeTypeIndex], lowestMPDrepRateIndex[mimeTypeIndex], bandwithList, segmentDuration*1000, debugLog, glob.DebugFile, &thrList, thr, preRepRate)
		case glob.BBA2Alg_AV:
			repRate = algo.BBA2(bufferLevel, maxBufferLevel, highestMPDrepRateIndex[mimeTypeIndex], lowestMPDrepRateIndex[mimeTypeIndex], bandwithList, segmentDuration*1000, debugLog, glob.DebugFile, &thrList, thr, preRepRate, segmentNumber, &BBA2DataStruct)
		case glob.BBA2Alg_AVXL_base:
			repRate = algo.BBA2(bufferLevel, maxBufferLevel, highestMPDrepRateIndex[mimeTypeIndex], lowestMPDrepRateIndex[mimeTypeIndex], bandwithList, segmentDuration*1000, debugLog, glob.DebugFile, &thrList, thr, preRepRate, segmentNumber, &BBA2DataStruct)
		case glob.BBA2Alg_AVXL_rate:
			repRate = algo.BBA2(bufferLevel, maxBufferLevel, highestMPDrepRateIndex[mimeTypeIndex], lowestMPDrepRateIndex[mimeTypeIndex], bandwithList, segmentDuration*1000, debugLog, glob.DebugFile, &thrList, thr, preRepRate, segmentNumber, &BBA2DataStruct)
		case glob.BBA2Alg_AVXL_double:
			repRate = algo.BBA2(bufferLevel, maxBufferLevel, highestMPDrepRateIndex[mimeTypeIndex], lowestMPDrepRateIndex[mimeTypeIndex], bandwithList, segmentDuration*1000, debugLog, glob.DebugFile, &thrList, thr, preRepRate, segmentNumber, &BBA2DataStruct)
		}
		logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", adapt+" has choosen rep_Rate "+strconv.Itoa(repRate)+" @ a rate of "+strconv.Itoa(bandwithList[repRate]/glob.Conversion1000))

		postRepRate := repRate
		if preRepRate != postRepRate {
			from := abrqlog.NewRepresentation()
			from.ID = strconv.Itoa(preRepRate)
			from.Bitrate = int64(bandwithList[preRepRate] / glob.Conversion1000)
			to := abrqlog.NewRepresentation()
			to.ID = strconv.Itoa(postRepRate)
			to.Bitrate = int64(bandwithList[postRepRate] / glob.Conversion1000)
			abrqlog.MainTracer.Switch(mimeTypesMediaType[mimeTypeIndex], from, to)
		}

		//Increase the segment number
		segmentNumber++

		// break out if we have downloaded all of our segments
		if segmentDurationTotal+(segmentDuration*glob.Conversion1000) > streamDuration {
			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "We have downloaded all segments at the end of the streamLoop - segment total: "+strconv.Itoa(segmentDurationTotal)+"  current segment duration: "+strconv.Itoa(segmentDuration*glob.Conversion1000)+" gives a total of:  "+strconv.Itoa(segmentDurationTotal+(segmentDuration*glob.Conversion1000)))

			if mimeTypeIndex == len(mimeTypes)-1 {
				// save the current log
				streamStructs[mimeTypeIndex].MapSegmentLogPrintout = mapSegmentLogPrintout
				// get the logs for all adaptationSets
				for thisMimeTypeIndex := range mimeTypes {
					mapSegmentLogPrintouts = append(mapSegmentLogPrintouts, streamStructs[thisMimeTypeIndex].MapSegmentLogPrintout)
				}

				playhead := abrqlog.NewPlayheadStatus()
				playhead.PlayheadTime = time.Duration(playPosition) * time.Millisecond
				abrqlog.MainTracer.EndStream(playhead)

				return segmentNumber, mapSegmentLogPrintouts
			}
		}

		// save info for the next segment
		streaminfo := http.StreamStruct{
			SegmentNumber:         segmentNumber,
			CurrentURL:            OriginalURL,
			InitBuffer:            initBuffer,
			MaxBuffer:             maxBuffer,
			CodecName:             codecName,
			Codec:                 codec,
			UrlString:             urlString,
			UrlInput:              urlInput,
			MpdList:               mpdList,
			Adapt:                 adapt,
			MaxHeight:             maxHeight,
			IsByteRangeMPD:        isByteRangeMPD,
			StartTime:             startTime,
			NextRunTime:           nextRunTime,
			ArrivalTime:           arrivalTime,
			OldMPDIndex:           oldMPDIndex,
			NextSegmentNumber:     nextSegmentNumber,
			Hls:                   hls,
			HlsBool:               hlsBool,
			MapSegmentLogPrintout: mapSegmentLogPrintout,
			StreamDuration:        streamDuration,
			StreamSpeed:           streamSpeed,
			ExtendPrintLog:        extendPrintLog,
			HlsUsed:               hlsUsed,
			BufferLevel:           bufferLevel,
			SegmentDurationTotal:  segmentDurationTotal,
			Quic:                  quic,
			QuicBool:              quicBool,
			BaseURL:               OriginalBaseURL,
			DebugLog:              debugLog,
			AudioContent:          audioContent,
			RepRate:               repRate,
			BandwithList:          bandwithList,
			Profile:               profile,
		}
		streamStructs[mimeTypeIndex] = streaminfo

		bufferStats := abrqlog.NewBufferStats()
		bufferStats.PlayoutTime = time.Duration(bufferLevel) * time.Millisecond
		bufferStats.MaxTime = time.Duration(streamStructs[mimeTypeIndex].MaxBuffer) * time.Second
		abrqlog.MainTracer.UpdateBufferOccupancy(mimeTypesMediaType[mimeTypeIndex],
			bufferStats)

		playhead := abrqlog.NewPlayheadStatus()
		playhead.PlayheadTime = time.Duration(playPosition) * time.Millisecond
		abrqlog.MainTracer.PlayheadProgress(playhead)
	}
	//}

	// this gets the index for the next MPD and the segment number for the next chunk
	stopPlayer := false

	// get some new info
	for mimeTypeIndex := range mimeTypes {
		stopPlayer, oldMPDIndex, nextSegmentNumber = http.GetNextSegmentDuration(segmentDurationArray, segmentDuration*glob.Conversion1000, segmentDurationTotal, glob.DebugFile, streamStructs[mimeTypeIndex].DebugLog, segmentDurationArray[mpdListIndex], streamStructs[mimeTypeIndex].StreamDuration)
		streamStructs[mimeTypeIndex].OldMPDIndex = oldMPDIndex
		streamStructs[mimeTypeIndex].NextSegmentNumber = nextSegmentNumber
		// mapSegmentLogPrintouts[mimeTypeIndex] = streamStructs[mimeTypeIndex].MapSegmentLogPrintout
		mapSegmentLogPrintouts = append(mapSegmentLogPrintouts, streamStructs[mimeTypeIndex].MapSegmentLogPrintout)
	}

	//fmt.Println("streamLoop oldMPDIndex: ", stopPlayer)

	// stream the next chunk
	if !stopPlayer {
		segmentNumber, mapSegmentLogPrintouts = streamLoop(streamStructs, Noden, accountant, metricsLogger)
	}

	return segmentNumber, mapSegmentLogPrintouts

}
