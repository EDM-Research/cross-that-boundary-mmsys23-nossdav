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

package global

// Conversion1000 : divider for conversion from bit to kilobit, to megabit, etc
const Conversion1000 = 1000

// Conversion1024 : divider for conversion from bit to kilobit, to megabit, etc
const Conversion1024 = 1024

// DebugFileName : debug log name
var DebugFileName = "logFile"

// DebugFolder : debug log folder location
const DebugFolder = "./logs/"

// DebugTextFile : debug log file location
const DebugTextFile = "log_file"

// FileFormat : debug file format
const FileFormat = ".txt"

// DebugFile : debug log folder + file + FileFormat
var DebugFile = DebugFolder + DebugTextFile + FileFormat

// Contains metrics for post-run analysis
var MetricsLogFile = "metrics_log"
var MetricsLogLoctation = DebugFolder + MetricsLogFile + FileFormat

// DownloadFileStoreName : where to save the downloaded files
var DownloadFileStoreName = "./files/"

// LogDownload : where to save the log download text
const LogDownload = "logDownload.txt"

// RepRateCodecAVC : AVC constants for our encoder
const RepRateCodecAVC = "h264"

// RepRateCodecHEVC : HEVC constants for our encoder
const RepRateCodecHEVC = "h265"

// RepRateCodecVP9 : VP9 constants for our encoder
const RepRateCodecVP9 = "VP9"

// RepRateCodecAV1 : AV1 constants for our encoder
const RepRateCodecAV1 = "AV1"

// RepRateCodecAudio : Audio constants for our encoder
const RepRateCodecAudio = "audio/mp4"

// RepRateCodecVideo : Audio constants for our encoder
const RepRateCodecVideo = "video/mp4"

// ConventionalAlg : constants for our algorithms
const ConventionalAlg = "conventional"

// ProgressiveAlg : constants for our algorithms
const ProgressiveAlg = "progressive"

// ElasticAlg : constants for our algorithms
const ElasticAlg = "elastic"

// LogisticAlg : constants for our algorithms
const LogisticAlg = "logistic"

// MeanAverageAlg : constants for our algorithms
const MeanAverageAlg = "average"

// GeomAverageAlg : constants for our algorithms
const GeomAverageAlg = "geometric"

// EMWAAverageAlg : constants for our algorithms
const EMWAAverageAlg = "exponential"

// BBAAlg : test constants for our algorithms
const BBAAlg = "bba"

// Arno Verstraete's BBA1
const BBA1Alg_AV = "bba1"

// Cross-layer version
const BBA1Alg_AVXL = "bba1XL"

// Arno Verstraete's BBA2
const BBA2Alg_AV = "bba2"

// Cross-layer version
const BBA2Alg_AVXL_base = "bba2XL-base"

// Cross-layer version with rate adaption
const BBA2Alg_AVXL_rate = "bba2XL-rate"

// Cross-layer version with double segment prediction
const BBA2Alg_AVXL_double = "bba2XL-double"

// TestAlg : test constants for our algorithms
const TestAlg = "test"

// ArbiterAlg : constants for our algorithms
const ArbiterAlg = "arbiter"

// ---- Cross-layer algorithms ----
const MeanAverageXLAlg = "averageXL"
const MeanAverageRecentXLAlg = "averageRecentXL"

// HlsOff : constants for HLS
const HlsOff = "off"

// HlsOn : constants for HLS
const HlsOn = "on"

// TrueBool : true string for booleans
const TrueBool = "true"

// FalseBool : false string for booleans
const FalseBool = "false"

// GetHeaderOff : constants for getHeader
const GetHeaderOff = "off"

// GetHeaderOn : constants for getHeader
const GetHeaderOn = "on"

// GetHeaderOnline : constants for getHeader
const GetHeaderOnline = "online"

// GetHeaderOffline : constants for getHeader
const GetHeaderOffline = "offline"

// URLName : parameter variables
const URLName = "url"

// ConfigName : parameter variables
const ConfigName = "config"

// DebugName : parameter variables
const DebugName = "debug"

// CodecName : parameter variables
const CodecName = "codec"

// MaxHeightName : parameter variables
const MaxHeightName = "maxHeight"

// NumSegmentsName : parameter variables
const NumSegmentsName = "numSegments"

// StreamDurationName : parameter variables
const StreamDurationName = "streamDuration"

// StreamSpeedName : parameter variables
const StreamSpeedName = "streamSpeed"

// PrintHeaderName : parameter variables
const PrintHeaderName = "printHeader"

// MaxBufferName : parameter variables
const MaxBufferName = "maxBuffer"

// InitBufferName : parameter variables
const InitBufferName = "initBuffer"

// AdaptName : parameter variables
const AdaptName = "adapt"

// FileStoreName : parameter variables
const FileStoreName = "outputFolder"

// StoreFiles : parameter variables
const StoreFiles = "storeDASH"

// StoreFilesOff : constants for storing files
const StoreFilesOff = "off"

// StoreFilesOn : constants for storing files
const StoreFilesOn = "on"

// TerminalPrintName : parameter variables
const TerminalPrintName = "terminalPrint"

// CollabPrintName : parameter variables
const CollabPrintName = "serveraddr"

// HlsName : parameter variables
const HlsName = "hls"

// QuicName : parameter variables
const QuicName = "quic"

// AppName : parameter variables
const AppName = "goDASH"

// ExpRatioName : parameter variables
const ExpRatioName = "expRatio"

// GetHeaderName : print header variables
const GetHeaderName = "getHeaders"

// RepRateBaseURL : used for determining if byte range MPD
const RepRateBaseURL = ""

// ByteRangeString : string for byte_range
const ByteRangeString = "_byte_range"

// DebugOff : constants for debug
const DebugOff = "off"

// DebugOn : constants for debug
const DebugOn = "on"

// TerminalPrintOff : constants for print
const TerminalPrintOff = "off"

// TerminalPrintOn : constants for print
const TerminalPrintOn = "on"

// CollabPrintOff : parameter variables
const CollabPrintOff = "off"

// CollabPrintOn : parameter variables
const CollabPrintOn = "on"

// QuicOff : constants for quic
const QuicOff = "off"

// QuicOn : constants for Extend
const QuicOn = "on"

// UseTestBedName : parameter variables
const UseTestBedName = "useTestbed"

// UseTestBedOff : constants for useTest
const UseTestBedOff = "off"

// UseTestBedOn : constants for useTest
const UseTestBedOn = "on"

// HTTPcertLocation : location of the http cert
const HTTPcertLocation = "http/certs/cert.pem"

// HTTPkeyLocation : location of the http key
const HTTPkeyLocation = "http/certs/key.pem"

// QoEName : parameter variables
const QoEName = "QoE"

// QoEOff : constants for QoE
const QoEOff = "off"

// QoEOn : constants for QoE
const QoEOn = "on"

// P1203maxWidth : P1203 standard max Width
const P1203maxWidth = 1920

// P1203maxHeight : P1203 standard max Height
const P1203maxHeight = 1080

// P1203exec : executable for P1203
const P1203exec = "p1203-standalone"

// InsecureSSL :  "Accept/Ignore all server SSL certificates"
const InsecureSSL = true

// Serv : port for the server
const Serv = "serverPort"

// Client : port for the "client"
const Client = "clientPort"

// headers for the print log

// SegNum : header for
const SegNum = "Seg_#"

// ArrTime : header for
const ArrTime = "Arr_time"

// DelTime : header for
const DelTime = "Del_Time"

// StallDur : header for
const StallDur = "Stall_Dur"

// RepLevel : header for
const RepLevel = "Rep_Level"

// DelRate : header for
const DelRate = "Del_Rate"

// ActRate : header for
const ActRate = "Act_Rate"

// ByteSize : header for
const ByteSize = "Byte_Size"

// BuffLevel : header for
const BuffLevel = "Buff_Level"

// AlgoHeader : header for
const AlgoHeader = "Algorithm"

// SegDurHeader : header for
const SegDurHeader = "Seg_Dur"

// CodecHeader : header for
const CodecHeader = "Codec"

// HeightHeader : header for
const HeightHeader = "Width"

// WidthHeader : header for
const WidthHeader = "Height"

// FpsHeader : header for
const FpsHeader = "FPS"

// PlayHeader : header for
const PlayHeader = "Play_Pos"

// RttHeader : header for
const RttHeader = "RTT"

// SegReplaceHeader : header for
const SegReplaceHeader = "Seg_Repl"

// HTTPProtocolHeader : header for
const HTTPProtocolHeader = "Protocol"

// QOE

// P1203Header : header for
const P1203Header = "P.1203"

// ClaeHeader : header for
const ClaeHeader = "Clae"

// DuanmuHeader : header for
const DuanmuHeader = "Duanmu"

// YinHeader : header for
const YinHeader = "Yin"

// YuHeader : header for
const YuHeader = "Yu"
