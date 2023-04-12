package HelperFunctions

import (
	"io"
	"net/http"
	"os"

	abrqlog "github.com/uccmisl/godash/qlog"
)

//DownloadFile This function downloads file at given url
func DownloadFile(filepath string, url string) error {

	// TODO better media type?
	abrqlog.MainTracer.Request(abrqlog.MediaTypeOther, url, "")

	//download data
	response, err := http.Get(url)

	if err != nil {
		return err
	}

	abrqlog.MainTracer.RequestUpdate(url, response.ContentLength)

	defer response.Body.Close()

	//create file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}

	defer out.Close()

	//Write to file
	_, err = io.Copy(out, response.Body)
	return err
}
