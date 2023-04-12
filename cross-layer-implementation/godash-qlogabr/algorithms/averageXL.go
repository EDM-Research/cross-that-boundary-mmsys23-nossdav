/**

Cross-layer version of the MeanAverage algorithm

*/

package algorithms

import (
	"github.com/uccmisl/godash/crosslayer"
)

func MeanAverageXLAlgo(XLaccountant *crosslayer.CrossLayerAccountant, thrList *[]int, newThr int, repRate *int, bandwithList []int, lowestMPDrepRateIndex int) {
	var average float64

	*thrList = append(*thrList, newThr)

	//if there is not enough throughtputs in the list, can't calculate the average
	if len(*thrList) < 2 {
		//if there is not enough throughtput, we call selectRepRate() with the newThr
		*repRate = SelectRepRateWithThroughtput(newThr, bandwithList, lowestMPDrepRateIndex)
		return
	}

	// average of the last throughtputs
	meanAverage(*thrList, &average)
	xlaverage := XLaccountant.GetAverageThroughput()

	/*
		fmt.Println("------------------------")
		fmt.Println("AVERAGE: ", int64(average))
		fmt.Println("AVERAGEXL: ", int64(xlaverage))
		fmt.Println("DIFF: ", int64(xlaverage-average))
		fmt.Println("------------------------")
	*/

	//We select the reprate with the calculated throughtput
	*repRate = SelectRepRateWithThroughtput(int(xlaverage), bandwithList, lowestMPDrepRateIndex)
}
