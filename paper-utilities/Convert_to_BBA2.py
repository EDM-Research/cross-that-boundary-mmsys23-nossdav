import sys
import xml.etree.ElementTree as ET
import glob
import os

ET.register_namespace('', 'urn:mpeg:dash:schema:mpd:2011')

def getAdaptationSet(root):
    for child in root:
        if child.tag.endswith("Period"):
            for pchild in child:
                if pchild.tag.endswith("AdaptationSet"):
                    return pchild
    print("Could not find AdaptationSet")
    exit(1)

if len(sys.argv) < 2:
    print("Usage: Convert_to_BAA2.py mpdLocation")
    exit()

mpdPath = sys.argv[1]
rootPathSplit = mpdPath.split('/')
rootPath = mpdPath[:-len(rootPathSplit[-1])]

print("MPD: ", mpdPath)
print("Root dir: ", rootPath)

tree = ET.parse(mpdPath)
root = tree.getroot()

adaps = getAdaptationSet(root)

videoTemplate = ""
startSegmentNumber = 0
timescale = ""
initialization = ""
duration = ""

for rep in adaps:
    if rep.tag.endswith("SegmentTemplate"):
        videoTemplate = rep.attrib["media"]
        startSegmentNumber = int(rep.attrib["startNumber"])
        timescale = rep.attrib["timescale"]
        initialization = rep.attrib["initialization"]
        duration = rep.attrib["duration"]

# For every representation, find all segments and calculate maxAvgRatio, make chunksize list
for rep in adaps:
    if rep.tag.endswith("Representation"):
        repID = rep.attrib["id"]
        print(repID)
        repBandWidth = rep.attrib["bandwidth"]

        repPath = rootPath + videoTemplate
        repPath = repPath.replace('$Bandwidth$', repBandWidth).replace('$Number$', '*')
        representationMedia = videoTemplate.replace('$Bandwidth$', repBandWidth)
        repinitialization = initialization.replace('$Bandwidth$', repBandWidth)

        chunkList = []

        nSegments = len(glob.glob(repPath))

        sum = 0
        max = 0

        for i in range(startSegmentNumber, nSegments + 1):
            segPath = repPath.replace('*', str(i))
            #print(segPath)
            byteSize = os.path.getsize(segPath)
            bitSize = byteSize * 8
            chunkList.append(bitSize)

            sum += bitSize
            if bitSize > max:
                max = bitSize
        print(i, len(chunkList))
        avg = float(sum) / float(i)
        print("avg: ", avg)
        maxAvgRatio = float(max) / avg

        # Add ratio to XML
        rep.set("maxAvgRatio", str(maxAvgRatio))

        chunkListStr = ""
        for chunk in chunkList:
            chunkListStr += str(chunk) + ','
        # Remove last comma
        chunkListStr = chunkListStr[:-1]

        # Add SegmentTemplate to representation XML
        segtempEl = ET.Element("SegmentTemplate")
        segtempEl.attrib["timescale"] = timescale
        segtempEl.attrib["media"] = representationMedia
        segtempEl.attrib["startNumber"] = str(startSegmentNumber)
        segtempEl.attrib["duration"] = duration
        segtempEl.attrib["initialization"] = repinitialization

        rep.append(segtempEl)

        # Add chunks to XML
        chunksEl = ET.Element("chunks")
        chunksEl.text = chunkListStr

        rep.append(chunksEl)

        print(repBandWidth, maxAvgRatio)

newMPD = mpdPath[:-4] + 'BBA2' + '.mpd'
print(newMPD)

tree.write(newMPD, default_namespace='')
