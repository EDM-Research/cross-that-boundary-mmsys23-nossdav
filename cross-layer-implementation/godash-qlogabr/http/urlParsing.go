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

package http

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/francoispqt/gojay"
	"github.com/uccmisl/godash/logging"
	"github.com/uccmisl/godash/utils"

	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/uccmisl/godash/P2Pconsul"
	"github.com/uccmisl/godash/P2Pconsul/HelperFunctions"
	glob "github.com/uccmisl/godash/global"

	"github.com/cavaliercoder/grab"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	quiclogging "github.com/lucas-clemente/quic-go/logging"
	"github.com/lucas-clemente/quic-go/qlog"

	xlayer "github.com/uccmisl/godash/crosslayer"
	abrqlog "github.com/uccmisl/godash/qlog"
)

// *** Copied from "github.com/lucas-clemente/quic-go/internal/utils"

type bufferedWriteCloser struct {
	*bufio.Writer
	io.Closer
}

// NewBufferedWriteCloser creates an io.WriteCloser from a bufio.Writer and an io.Closer
func NewBufferedWriteCloser(writer *bufio.Writer, closer io.Closer) io.WriteCloser {
	return &bufferedWriteCloser{
		Writer: writer,
		Closer: closer,
	}
}

func (h bufferedWriteCloser) Close() error {
	if err := h.Writer.Flush(); err != nil {
		return err
	}
	return h.Closer.Close()
}

// ***

// Noden consul node
var Noden P2Pconsul.NodeUrl

// SetNoden set the consul node
func SetNoden(node P2Pconsul.NodeUrl) {
	Noden = node
}

var client *http.Client = nil
var tr *http.Transport
var trQuic *http3.RoundTripper

var globAccountant *xlayer.CrossLayerAccountant = nil

// Sets the globAccountant to the given accountant object
func SetAccountant(acc *xlayer.CrossLayerAccountant) {
	globAccountant = acc
}

// getHTTPClient:
func GetHTTPClient(quicBool bool, debugFile string, debugLog bool, useTestbedBool bool) (*http.Transport, *http.Client, *http3.RoundTripper) {

	if client != nil {
		return tr, client, trQuic
	}

	var cert tls.Certificate
	var caCertPool = x509.NewCertPool()
	var caCert []byte
	var config *tls.Config
	var quicConfig *tls.Config

	// if we are using the mininet testbed
	if useTestbedBool {
		logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Testbed in use")
		// where are we
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Unable to determine executable location for testbed server certs")
			log.Fatal(err)
		}

		// Read the key pair to create certificate
		cert, err = tls.LoadX509KeyPair(dir+"/"+glob.HTTPcertLocation, dir+"/"+glob.HTTPkeyLocation)
		if err != nil {
			log.Println("Unable to load X509 key and cert")
			log.Fatal(err)
		}
		logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "loading X509 key and cert: "+dir+"/"+glob.HTTPcertLocation+" "+dir+"/"+glob.HTTPkeyLocation)

		// Create a CA certificate pool and add cert.pem to it
		caCert, err = ioutil.ReadFile(dir + "/" + glob.HTTPcertLocation)
		if err != nil {
			log.Println("Unable to read X509 cert")
			log.Fatal(err)
		}
		logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "reading X509 cert")

		// add cert to pool
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "No certs appended, using system certs only")
		}
	}

	// TODO: remove, we now receive this channel from upper calls
	//qlogEventChan := make(chan qlog.Event)

	// if we want to use quic
	if quicBool {
		qconf := quic.Config{}
		//qconf.KeepAlive = true
		qconf.Tracer = qlog.NewTracer(func(_ quiclogging.Perspective, connID []byte) io.WriteCloser {
			filename := fmt.Sprintf("logs/client_%x.qlog", connID)
			//filename := "logs/client.qlog"
			f, err := os.Create(filename)
			//f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Creating qlog file %s.\n", filename)
			return NewBufferedWriteCloser(bufio.NewWriter(f), f)
		},
			globAccountant.EventChannel,
		)
		//go printQlogEvents(qlogEventChan)
		//accountant := xlayer.CrossLayerAccountant{EventChannel: qlogEventChan}
		//accountant.Listen()

		// if we are not using the terstbed
		if !useTestbedBool {
			trQuic = &http3.RoundTripper{
				TLSClientConfig: &tls.Config{
					RootCAs:            caCertPool,
					InsecureSkipVerify: glob.InsecureSSL,
				},
				QuicConfig: &qconf,
			}
			defer trQuic.Close()
			client = &http.Client{
				Transport: trQuic,
			}
		} else {

			// lets try the testbed using IETF quic
			// set up the config
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "creating tls config for quic")
			quicConfig = &tls.Config{
				// use insecure SSL - if needed only use during internal tests
				// this is set statically in the globalVar.go file (set to true if needed)
				InsecureSkipVerify: glob.InsecureSSL,
				RootCAs:            caCertPool,
				Certificates:       []tls.Certificate{cert},
			}
			// set up our http transport
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "creating our http transport using our tls config for quic")

			trQuic = &http3.RoundTripper{TLSClientConfig: quicConfig, QuicConfig: &qconf, DisableCompression: true}
			// set up the client
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "creating our client using our http transport and our tls config for quic")
			client = &http.Client{Transport: trQuic}
		}
		// otherwise use a normal-ish HTTP client
	} else {
		// set up a secure-ish http client with out quic
		if useTestbedBool {
			// set up the config
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "creating tls config")
			config = &tls.Config{
				// use insecure SSL - if needed only use during internal tests
				// this is set statically in the globalVar.go file (set to true if needed)
				InsecureSkipVerify: glob.InsecureSSL,
				RootCAs:            caCertPool,
				Certificates:       []tls.Certificate{cert},
			}
			// set up our http transport
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "creating our http transport using our tls config")
			tr = &http.Transport{TLSClientConfig: config}
			// set up the client
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "creating our client using our http transport and our tls config")
			client = &http.Client{Transport: tr}

		} else {
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "setup default client but with a defined ssl security check")
			config = &tls.Config{
				// use insecure SSL - if needed only use during internal tests
				// this is set statically in the globalVar.go file (set to true if needed)
				InsecureSkipVerify: glob.InsecureSSL,
			}
			tr = &http.Transport{TLSClientConfig: config}
			client = &http.Client{Transport: tr}
		}
	}

	return tr, client, trQuic

}

// getURLBody :
// * get the response body of the url
// * calculate the rtt
// * return the response body and the rtt
func getURLBody(url string, isByteRangeMPD bool, startRange int, endRange int, quicBool bool, debugFile string, debugLog bool, useTestbedBool bool, returnContentLengthOnly bool, ctx context.Context) (io.ReadCloser, time.Duration, string, int, int) {

	var client *http.Client
	var err error
	// var tr *http.Transport
	// var trQuic *http3.RoundTripper
	var contentLen = 0

	// assign the protocols for this client
	_, client, _ = GetHTTPClient(quicBool, debugFile, debugLog, useTestbedBool)

	// request the url
	logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Get the url "+url)
	//req, err := http.NewRequest("GET", url, nil)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)

	if err != nil {
		fmt.Println(err)
		fmt.Println("the URL " + url + " doesn't match with anything")
		abrqlog.MainTracer.AbortRequest(url)
		// stop the app
		utils.StopApp()
	}

	// logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "get the rtt "+url)
	// if quicBool {
	// 	// define a recursive call
	// 	var recuriveQuicCall func(int)

	// 	// a recursive function to check for connection drops
	// 	recuriveQuicCall = func(count int) {
	// 		// check the connection using quic
	// 		_, err := trQuic.RoundTrip(req)
	// 		if count > 5 {
	// 			fmt.Println("Unable to connect to the URL " + url + " on the testbed using quic")
	// 			fmt.Println(err)
	// 			// stop the app
	// 			os.Exit(3)
	// 		} else if err != nil {
	// 			// lets sleep for 100 milliseconds
	// 			time.Sleep(1000 * time.Millisecond)
	// 			fmt.Println(count)
	// 			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "unable to connect to server, lets get the file again "+string(count))
	// 			recuriveQuicCall(count + 1)
	// 		}
	// 		return
	// 	}
	// 	// lets call this 5 times just incase we can't reach due to no network connection
	// 	recuriveQuicCall(1)

	// } else if useTestbedBool {
	// 	// define a recursive call
	// 	var recuriveTestbedCall func(int)

	// 	// a recursive function to check for connection drops
	// 	recuriveTestbedCall = func(count int) {
	// 		// use our new transport for calculating the rtt
	// 		_, err := tr.RoundTrip(req)
	// 		if count > 5 {
	// 			fmt.Println("Unable to connect to the URL " + url + " on the testbed")
	// 			fmt.Println(err)
	// 			// stop the app
	// 			os.Exit(3)
	// 		} else if err != nil {
	// 			// lets sleep for 100 milliseconds
	// 			time.Sleep(1000 * time.Millisecond)
	// 			fmt.Println(count)
	// 			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "unable to connect to server, lets get the file again "+string(count))
	// 			recuriveTestbedCall(count + 1)
	// 		}
	// 		return
	// 	}
	// 	// lets call this 5 times just incase we can't reach due to no network connection
	// 	recuriveTestbedCall(1)

	// } else {
	// 	// define a recursive call
	// 	var recuriveDefaultCall func(int)

	// 	// a recursive function to check for connection drops
	// 	recuriveDefaultCall = func(count int) {
	// 		// use http default transport for calculating the rtt
	// 		_, err := http.DefaultTransport.RoundTrip(req)
	// 		if count > 5 {
	// 			fmt.Println("Unable to connect to the URL " + url + " using default settings")
	// 			fmt.Println(err)
	// 			// stop the app
	// 			os.Exit(3)
	// 		} else if err != nil {
	// 			// lets sleep for 100 milliseconds
	// 			time.Sleep(1000 * time.Millisecond)
	// 			fmt.Println(count)
	// 			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "unable to connect to server, lets get the file again "+string(count))
	// 			recuriveDefaultCall(count + 1)
	// 		}
	// 		return
	// 	}
	// 	// lets call this 5 times just incase we can't reach due to no network connection
	// 	recuriveDefaultCall(1)
	// }

	// add the byte ranges, if byte-range
	if isByteRangeMPD {
		byteRange := "bytes=" + strconv.Itoa(startRange) + "-" + strconv.Itoa(endRange)
		req.Header.Add("Range", byteRange)
	}

	var resp *http.Response

	// if we want to use quic
	// determine the rtt for this segment
	start := time.Now()
	//request the URL using the client
	resp, err = client.Do(req)
	// get rtt
	end := time.Now()
	rtt := end.Sub(start)

	abrqlog.MainTracer.RTT.UpdateRTT(rtt, end)
	abrqlog.MainTracer.UpdatedMetrics(abrqlog.MainTracer.RTT)

	if err != nil {
		fmt.Println(err)
		fmt.Println("the URL " + url + " doesn't match with anything")
		abrqlog.MainTracer.AbortRequest(url)
		// stop the app
		utils.StopApp()
	}

	// get protocol version
	protocol := resp.Proto
	status := resp.StatusCode

	logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "URL is : "+url)
	logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Protocol is : "+protocol)

	//Check if the GET method has sent a status code equal to 200
	if resp.StatusCode != http.StatusOK && !isByteRangeMPD {
		// add this to the debug log
		logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "The URL returned a non status okay error code: "+strconv.Itoa(resp.StatusCode))
		abrqlog.MainTracer.AbortRequest(url)
		// stop the app
		utils.StopApp()
	}
	//fmt.Println("len : ", resp.ContentLength)

	// return the response body
	return resp.Body, rtt, protocol, contentLen, status

}

// getURLProgressively :
// * get the response body of the url
// * calculate the rtt and throughtput for the download per second
// * return the rtt
func getURLProgressively(url string, isByteRangeMPD bool, startRange int, endRange int, fileLocation string) time.Duration {

	var thrPerSecond []int64

	// set up a http client
	client := grab.NewClient()
	// request the url and save to a file location
	req, err := grab.NewRequest(fileLocation, url)
	// if there is an error, stop the app
	if err != nil {
		fmt.Println(err)
		fmt.Println("the URL " + url + " doesn't match with anything")
		abrqlog.MainTracer.AbortRequest(url)
		// stop the app
		utils.StopApp()
	}

	// determine the rtt for this segment
	start := time.Now()
	if _, err := http.DefaultTransport.RoundTrip(req.HTTPRequest); err != nil {
		log.Fatal(err)
	}
	// get rtt
	rtt := time.Since(start)
	//fmt.Printf("grab RTT in %dms for %s\n", rtt, url)

	// add the byte ranges, if byte-range
	if isByteRangeMPD {
		byteRange := "bytes=" + strconv.Itoa(startRange) + "-" + strconv.Itoa(endRange)
		req.HTTPRequest.Header.Add("Range", byteRange)
	}

	//request the URL using the client
	resp := client.Do(req)

	// start UI loop, (maybe we should put 1 instead of 1000 to have it in millisecond)
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()

	// Check if the download has finished or not
	//start = time.Now()
	for !resp.IsComplete() {
		select {
		case <-t.C:
			/*
				fmt.Printf("transferred %v / %v bytes (%.2f%%) in %dms\n",
					resp.BytesComplete(),
					resp.Size,
					100*resp.Progress(), time.Since(start)/1000000)
			*/
			thrPerSecond = append(thrPerSecond, resp.BytesComplete())

		case <-resp.Done:
			// download is complete
			/*
				fmt.Printf("transferred %v / %v bytes (%.2f%%) in %dms\n",
					resp.BytesComplete(),
					resp.Size,
					100*resp.Progress(), time.Since(start)/1000000)
			*/
			thrPerSecond = append(thrPerSecond, resp.BytesComplete())
			break
		}
	}
	// check for errors
	if err := resp.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		abrqlog.MainTracer.AbortRequest(url)
		// stop the app
		utils.StopApp()
	}

	/* We can't use this as progressive has a different status code
	//Check if the GET method has sent a status code equal to 200
	if resp.HTTPResponse.StatusCode != http.StatusOK && !isByteRangeMPD {
		// add this to the debug log
		fmt.Println("The URL returned a non status okay error code: " + strconv.Itoa(resp.HTTPResponse.StatusCode))
		// stop the app
		utils.StopApp()
	}
	*/

	// return the rtt
	return rtt

}

// GetURLByteRangeBody :
// * get the response body of the url and return an io.ReadCloser
// * based on byte-ranges
func GetURLByteRangeBody(url string, startRange int, endRange int) (io.ReadCloser, time.Duration) {

	// set up a http client
	client := &http.Client{}
	// request the url
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println(err)
		fmt.Println("the URL " + url + " doesn't match with anything")
		abrqlog.MainTracer.AbortRequest(url)
		// stop the app
		utils.StopApp()
	}

	//req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	start := time.Now()
	if _, err := http.DefaultTransport.RoundTrip(req); err != nil {
		log.Fatal(err)
	}
	// get rtt
	rtt := time.Since(start)

	// add the byte ranges
	byteRange := "bytes=" + strconv.Itoa(startRange) + "-" + strconv.Itoa(endRange-1)
	req.Header.Add("Range", byteRange)

	//request the URL using the client
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		fmt.Println("the URL " + url + " doesn't match with anything")
		abrqlog.MainTracer.AbortRequest(url)
		// stop the app
		utils.StopApp()
	}

	//Check if the GET method has sent a status code equal to 200
	if resp.StatusCode != http.StatusOK {
		// add this to the debug log
		fmt.Println("The URL returned a non status okay error code: " + strconv.Itoa(resp.StatusCode))
		abrqlog.MainTracer.AbortRequest(url)
		// stop the app
		utils.StopApp()
	}
	//fmt.Println("len : ", resp.ContentLength)

	// return the response body
	return resp.Body, rtt

}

// GetURL :
// * return the content of the body of the url
func GetURL(url string, isByteRangeMPD bool, startRange int, endRange int, quicBool bool, debugFile string, debugLog bool, useTestbedBool bool) ([]byte, time.Duration, string) {

	byteRangeString := ""
	if startRange != endRange {
		byteRangeString = fmt.Sprint(startRange) + "-" + fmt.Sprint(endRange)
	}
	abrqlog.MainTracer.Request(abrqlog.MediaTypeOther, url, byteRangeString)

	ctx2 := context.Background()

	// get the response body and rtt for this url
	responseBody, rtt, protocol, _, status := getURLBody(url, isByteRangeMPD, startRange, endRange, quicBool, debugFile, debugLog, useTestbedBool, false, ctx2)

	// Lets read from the http stream and not create a file to store the body
	body, err := ioutil.ReadAll(responseBody)
	//bodyString := string(body)
	if err != nil {
		fmt.Println("Unable to read from url")
		fmt.Println("Statuscode:", status)
		abrqlog.MainTracer.AbortRequest(url)
		// stop the app
		utils.StopApp()
	}

	abrqlog.MainTracer.RequestUpdate(url, int64(len(body)))

	// close the responseBody
	responseBody.Close()

	// return the body of the responseBody
	return body, rtt, protocol
}

// GetRepresentationBaseURL :
// * get BaseURL for byte-range MPD
func GetRepresentationBaseURL(mpd MPD, currentMPDRepAdaptSet int) string {
	return mpd.Periods[0].AdaptationSet[currentMPDRepAdaptSet].Representation[0].BaseURL
}

func GetRepresentationMimeType(mpd MPD, currentMPDRepAdaptSet int) string {
	return mpd.Periods[0].AdaptationSet[currentMPDRepAdaptSet].Representation[0].MimeType
}

// JoinURL :
/*
 * func joinURL(baseURL string, append string) string
 *
 * join components of urls together
 * return the URL
 */
func JoinURL(baseURL string, append string, debugLog bool) string {

	// if "append" already contains "http", then do nothing
	if !(strings.Contains(append, "http")) {
		// get the base of the current url
		base := path.Base(baseURL)
		// replace this base url with the required file string
		urlHeaderString := strings.Replace(baseURL, base, append, -1)
		//logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "complete URL: "+urlHeaderString)

		// return the new url
		return urlHeaderString
	}
	// return the new url
	return append
}

// GetFile :
/*
 * Function getFile :
 * get the provided file from the online HTTP server and save to folder
 */
func GetFile(currentURL string, fileBaseURL string, fileLocation string, isByteRangeMPD bool, startRange int, endRange int,
	segmentNumber int, segmentDuration int, addSegDuration bool, quicBool bool, debugFile string, debugLog bool,
	useTestbedBool bool, repRate int, saveFilesBool bool, AudioByteRange bool, profile string, mediaType abrqlog.MediaType,
	ctx context.Context) (time.Duration, int, string, string, float64, int) {

	// create the string where we want to save this file
	var createFile string

	// join the new file location to the base url
	urlHeaderString := JoinURL(currentURL, fileBaseURL, debugLog)

	logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "get file from URL: "+urlHeaderString+"\n")

	if urlHeaderString == "" {
		fmt.Println("null urlHeader")
	}

	// we only want the base file of the url (sometimes the segment media url has multiple folders)
	base := path.Base(fileBaseURL)

	// we need to create a file to save for the byte-range content
	// but only for the video byte range and not audio byte range
	// if isByteRangeMPD && !AudioByteRange {
	// for now, lets just save each segment
	if isByteRangeMPD {
		s := strings.Split(base, ".")
		base = s[0] + "_segment" + strconv.Itoa(segmentNumber) + ".m4s"
	}

	// create the new file location, or not
	if !strings.Contains(base, profile) && (addSegDuration || AudioByteRange) {
		createFile = fileLocation + "/" + strconv.Itoa(segmentDuration) + "sec_" + profile + "_" + base
	} else {
		createFile = fileLocation + "/" + base
	}

	byteRangeString := ""
	if startRange != endRange {
		byteRangeString = fmt.Sprint(startRange) + "-" + fmt.Sprint(endRange)
	}
	abrqlog.MainTracer.Request(mediaType, urlHeaderString, byteRangeString)

	//request the URL with GET
	body, rtt, protocol, _, status := getURLBody(urlHeaderString, isByteRangeMPD, startRange, endRange, quicBool, debugFile, debugLog, useTestbedBool, false, ctx)

	// read from the buffer
	var buf bytes.Buffer
	// duplicate the buffer incase I need it later
	tee := io.TeeReader(body, &buf)
	myBytes, _ := ioutil.ReadAll(tee)
	// get the size of this segment
	size := strconv.FormatInt(int64(len(myBytes)), 10)
	segSize, err := strconv.Atoi(size)
	if err != nil {
		logging.DebugPrint(debugFile, debugLog, "Error : ", "Cannot convert the size to an int when getting a file")
		abrqlog.MainTracer.AbortRequest(urlHeaderString)
		utils.StopApp()
	}

	abrqlog.MainTracer.RequestUpdate(urlHeaderString, int64(segSize))

	// get the P.1203 segSize (less the header)
	withoutHeaderVal := int64(segSize)

	// lets see if we can find this {0x00, 0x00, 0x00, 0x04, 0x68, 0xEF, 0xBC, 0x80}
	// in our segment
	src := []byte("0000000468EFBC80")
	dst := make([]byte, hex.DecodedLen(len(src)))
	n, err := hex.Decode(dst, src)
	if err != nil {
		log.Fatal(err)
	}
	// see if this value is in myBytes
	if bytes.Contains(myBytes, dst[:n]) {
		// get the index for our dst value
		mdatValueInt := bytes.Index(myBytes, dst[:n])
		// add 8 bits for header
		mdatValueInt += 8
		// get the file byte size less the header
		withoutHeaderVal = int64(segSize) - int64(mdatValueInt)
	}
	// determine the bitrate based on segment duration - multiply by 8 and divide by segment duration
	kbpsInt := ((withoutHeaderVal * 8) / int64(segmentDuration))
	// convert kbps to a float
	kbpsFloat := float64(kbpsInt) / glob.Conversion1024
	// convert to sn easier string value
	kbpsFloatStringVal := fmt.Sprintf("%3f", kbpsFloat)
	// log this value
	logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "HTTP body size is "+kbpsFloatStringVal)

	// if we want to save the streamed files
	// NOTICE (Arno Verstraete): this check was disabled for testing
	if saveFilesBool {
		//if false {

		// Restore the io.ReadCloser to it's original state, if needed
		body = ioutil.NopCloser(bytes.NewBuffer(myBytes))

		// save the file to the provided file location
		// write if not existing, append if existing
		out, err := os.OpenFile(createFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("*** " + createFile + " cannot be downloaded and written/append to file ***")
			utils.StopApp()
		}
		// save the file to the provided file location
		// out, err := os.Create(createFile)
		// if err != nil {
		// 	fmt.Println("*** " + createFile + " cannot be downloaded and written to file ***")
		// 	// stop the app
		// 	utils.StopApp()
		// }
		// defer out.Close()

		// Write the body to file
		_, err = io.Copy(out, body)
		if err != nil {
			fmt.Println("*** " + createFile + " cannot be saved ***")
			// stop the app
			utils.StopApp()
		}
	}

	logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Before consul update")
	//check if mode is collaborative or standard
	if Noden.ClientName != "off" && Noden.ClientName != "" {
		logging.DebugPrint(debugFile, debugLog, "DEBUG: consul client - ", Noden.ClientName)
		Noden.UpdateConsul(HelperFunctions.HashSha(urlHeaderString))
	}
	logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "After consul update")

	// close the body connection
	body.Close()

	return rtt, segSize, protocol, createFile, kbpsFloat, status
}

// GetFileProgressively :
/*
 * get the provided file from the online HTTP server and save to folder
 * get a 1-second piece of each file
 */
func GetFileProgressively(currentURL string, fileBaseURL string, fileLocation string, isByteRangeMPD bool, startRange int, endRange int, segmentNumber int, segmentDuration int, addSegDuration bool, debugLog bool, AudioByteRange bool, profile string) (time.Duration, int) {

	// create the string where we want to save this file
	var createFile string

	// join the new file location to the base url
	urlHeaderString := JoinURL(currentURL, fileBaseURL, debugLog)
	logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "get file from URL: "+urlHeaderString+"\n")

	if urlHeaderString == "" {
		fmt.Println("null urlHeader")
	}

	// we only want the base file of the url (sometimes the segment media url has multiple folders)
	base := path.Base(fileBaseURL)

	// we need to create a file to save for the byte-range content
	// if isByteRangeMPD && !AudioByteRange {
	if isByteRangeMPD {
		// for now, lets just save each segment
		s := strings.Split(base, ".")
		base = s[0] + "_segment" + strconv.Itoa(segmentNumber) + ".m4s"
	}

	// create the new file location, or not
	if addSegDuration && !strings.Contains(base, profile) {
		createFile = fileLocation + "/" + strconv.Itoa(segmentDuration) + "sec_" + profile + "_" + base
	} else {
		createFile = fileLocation + "/" + base
	}

	// save the file to the provided file location
	out, err := os.Create(createFile)
	if err != nil {
		fmt.Println("*** " + createFile + " cannot be downloaded ***")
		// stop the app
		utils.StopApp()
	}
	defer out.Close()

	//request the URL with GET
	rtt := getURLProgressively(urlHeaderString, isByteRangeMPD, startRange, endRange, createFile)

	fi, err := os.Stat(createFile)
	if err != nil {
		fmt.Println(err)
	}

	size := strconv.FormatInt(fi.Size(), 10)
	segSize, err := strconv.Atoi(size)
	if err != nil {
		logging.DebugPrint(glob.DebugFile, debugLog, "Error : ", "Cannot convert the size to an int when getting a file")
		utils.StopApp()
	}

	return rtt, segSize
}

func printQlogEvents(c chan qlog.Event) {
	for msg := range c {
		start := time.Now()
		fmt.Println("Received transport layer event")
		fmt.Println(msg.RelativeTime)
		msgJSON, _ := gojay.Marshal(msg)
		duration := time.Since(start)
		fmt.Println(string(msgJSON))
		fmt.Println("Time to decode qlog JSON: ", duration.Microseconds(), "µs")
	}
}
