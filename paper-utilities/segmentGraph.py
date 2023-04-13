import sys
import numpy as np
import matplotlib.pyplot as plt

logFile = sys.argv[1]
shaperLogfile = sys.argv[2]

mode = sys.argv[5]

bufflevelTime = []
bufflevel = []

segmentTime = []
segmentRate = []

simThroughPutTime = []
simThroughPut = []

maxbuffer = 0
maxbandwidth = 0

startTime = 0

fixSimThroughForConstantBitrate = True

stallsDetected = []

windowthroughput = []
windowthroughputTime = []

reservoir = []
reservoirTime = []

bbaSwitchTimes = []

chunksum = []
chunksumTime = []

with open(logFile) as f:
    for line in f.readlines():
        line = line.replace('\n','')
        split = line.split(' ')
        time = split[0]
        tag = split[1]
        value = split[2]

        match tag:
            case "BUFFERLEVEL":
                bufflevel.append(int(value)/1000)
                bufflevelTime.append(int(time)/1000)
            case "SegmentDownloadStart":
                segmentRate.append(int(value)/1000)
                segmentTime.append(int(time)/1000)
            case "SegmentArrived":
                segmentRate.append(int(value)/1000)
                segmentTime.append(int(time)/1000)
            case "HIGHESTBANDWIDTH":
                maxbandwidth = int(value)/1000
            case "BUFFERSIZE":
                maxbuffer = int(value)
            case "STARTTIME":
                startTime = int(value)
            case "STALLPREDICTOR":
                stallsDetected.append(int(time)/1000)
            case "WINDOWTHROUGHPUT":
                windowthroughput.append(int(value))
                windowthroughputTime.append(int(time)/1000)
            case "LOWERRESERVOIR":
                reservoir.append(int(value)/1000)
                reservoirTime.append(int(time)/1000)
            case "LOGICSWITCH":
                bbaSwitchTimes.append(int(time)/1000)
            case "CHUNKSUM":
                chunksum.append(int(value))
                chunksumTime.append(int(time)/1000)
            case _:
                continue

if mode == "stallprediction" or mode == "bba":
    with open(shaperLogfile) as f:
        for line in f.readlines():
            line = line.replace('\n','')
            split = line.split(' ')
            time = split[0]
            tag = split[1]
            value = split[2]
            match tag:
                case "SIMULATIONTHROUGHPUT":
                    simThroughPut.append(int(value))
                    simThroughPutTime.append((int(time)*1000 - startTime)/1000)

    print(simThroughPut, simThroughPutTime)

fig, ax1 = plt.subplots()
ax2 = plt.twinx()

ax1.plot(bufflevelTime, bufflevel, color="blue", label="Buffer occupancy (s)")
ax2.step(segmentTime, segmentRate, color="green", label="Chosen representation (kbps)")

if mode == "stallprediction" or mode == "bba":
    if len(simThroughPut) != 0:
        if fixSimThroughForConstantBitrate:
            simThroughPutTime.insert(0, 0)
            simThroughPut.insert(0, simThroughPut[0])
            simThroughPutTime.append(max(segmentTime))
            simThroughPut.append(simThroughPut[-1])
        ax2.step(simThroughPutTime, simThroughPut, color="purple", label="Simulated throughput (kbps)")

if mode == "test":
    ax2.plot(windowthroughputTime, windowthroughput, color="grey", label="Window throughput (kbps)", zorder=0)

if mode == "bba":
    ax1.plot(reservoirTime, reservoir, color="grey", label="Lower reservoir (s)")
    counter = 0
    for el in bbaSwitchTimes:
        if counter == 0:
            plt.axvline(el, ls="--", color="orange", lw=0.5, label = "BBA switched logic")
        else:
            plt.axvline(el, ls="--", color="orange", lw=0.5)
        counter += 1

if mode == "stallprediction":
    counter = 0
    for el in stallsDetected:
        if counter == 0:
            plt.axvline(el, ls="--", color="red", lw=0.5, label = "Stall predicted")
        else:
            plt.axvline(el, ls="--", color="red", lw=0.5)
        counter += 1

ax1.set_ylim([0,max(maxbuffer,max(bufflevel))])
if mode == "test":
    ax2.set_ylim([0,max(maxbandwidth, max(windowthroughput))])
else:
    ax2.set_ylim([0,maxbandwidth])
ax1.set_xlim([0,max(segmentTime)])

ax1.set_xlabel("Time (s)")
ax1.set_ylabel("Buffer occupancy (s)", color="blue")
ax2.set_ylabel("Bitrate (kbps)")

lines, labels = ax1.get_legend_handles_labels()
lines2, labels2 = ax2.get_legend_handles_labels()
ax2.legend(lines + lines2, labels + labels2, loc=0)

plt.title(sys.argv[3])

plt.tight_layout()
plt.savefig(sys.argv[4], dpi=200)

plt.clf()

'''if mode=="bba":
    plt.plot(chunksumTime, chunksum)
    plt.show()'''