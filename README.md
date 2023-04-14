# NOSSDAV '23 Artifact "Cross that boundary: Investigating the feasibility of cross-layer information sharing for enhancing ABR decision logic over QUIC"
This repository contains the necessary artifacts to reproduce the findings of the NOSSDAV 2023 paper submission "Cross that boundary: Investigating the feasibility of cross-layer information sharing for enhancing ABR decision logic over QUIC" by Herbots and Verstraete et al.

The instructions below are to reproduce our findings using the [Vegvisir QUIC-HTTP/3 testing framework](https://github.com/JorisHerbots/vegvisir/).

## Repository structure
- ``cross-layer-implementation`` contains our adjusted ``quic-go`` and ``GoDASH`` implementations
- ``tc-netem-shaper`` contains the network simulation script to be used as the shaper in Vegvisir
- ``paper-utilities`` contains utility scripts for Vegvisir and for preparing datasets
- ``paper-logs`` contains the logs, metrics and graphs produced by the test setup that were used in the paper
	- Contains multiple subfolders that represent individual test cases: ``godashcl-{ABR algorithm}-{dataset}-{segment size}__tc-netem-cl-paper__quic-go``
	- ``ABR algorithm`` matches with the paper algorithm names *bba2*, *bba2-cl* and *bba2-cl-double*
	- ``dataset`` is either *bbb* for "Big Buck Bunny", *ed* for "Elephants Dream" or *ofm* for "Of Forests and Men"
	- ``segment size`` is either 2s, 4s or 6s
	- Each test case contains three more subfolders ``client``, ``shaper`` and ``server`` containing logs pertaining to that respective entity   


## Instructions for reproducing the setup with Vegvisir and examining the results
**Setting up the testing framework**

1. Create an empty folder in which to reproduce the setup, we shall henceforth refer to this as ``root``
2. Navigate to ``root``
3. Clone and follow the [installation instructions over at the Vegvisir repository](https://github.com/JorisHerbots/vegvisir#installation)
	1. Enter the cloned ``vegvisir`` folder
	2. Follow the recommended instructions and make use of a virtual environment called ``venv``
		1. To make execution on ubuntu easier, install the following software ``apt install python-is-python3 python3.10-venv``
	3. Additionally install the following pip packages manually, we will need these for the scripts further down the line: ``pip install numpy matplotlib``
4. Navigate back to ``root`` and clone this repository ``git clone https://github.com/EDM-Research/cross-that-boundary-mmsys23-nossdav.git paper``
5. Create the required Docker containers for Vegvisir
	1. Navigate to ``paper/cross-layer-implementation`` and create a Docker image using the provided Dockerfile: ``docker build -t godashcl .``
	2. Navigate to ``paper/tc-netem-shaper`` and create a Docker image using the provided Dockerfile: ``docker build -t tc-netem-cl .``
6. Copy all Vegvisir environment scripts from ``root/paper/paper-utilities/vegvisir-scripts/`` to ``root/vegvisir/vegvisir/environments``, overwrite existing files
7. Copy ``root/paper/paper-utilities/segmentGraph.py`` to ``root/vegvisir/util/``
8. Navigate to ``root/vegvisir/util/`` and clone the ITU-p1203 standalone implementation: ``git clone https://github.com/itu-p1203/itu-p1203.git``
9. Navigate into the ``itu-p1203`` folder and install the implementation with ``pip install .``
	1. Make sure the virtual environment of step 3 is still enabled!
10. Copy the contents of ``root/paper/paper-utilities/vegvisir-configurations/`` to ``root/vegvisir``
11. [Increase the UDP receive buffer size](https://github.com/quic-go/quic-go/wiki/UDP-Receive-Buffer-Size/#non-bsd) (non-BSD instructions)
	1. *Note: This is volatile. To make this change permanent, add the configuration to ``/etc/sysctl.conf`` and reload the configuration with ``sudo sysctl --system``

**Preparing the dataset**  
This paper makes use of the [MPEG-DASH dataset provided by Lederer et al.](https://dash.itec.aau.at/dash-dataset/)

12. Navigate to ``root/`` and create an empty folder called ``datasets``, then navigate into ``root/datasets/``
13. Create three folders ``BigBuckBunny``, ``OfForestAndMen`` and ``ElephantsDream``
14. Retrieve the **2s**, **4s** and **6s** datasets in their respective folders of the following datasets (we recommend using ``wget --recursive --no-parent --reject="*index.html*" -nc -l 20 --continue {URL}``):
	1. [Big Buck Bunny](http://ftp.itec.aau.at/datasets/DASHDataset2014/BigBuckBunny/)
	2. [Of Forest And Men](http://ftp.itec.aau.at/datasets/DASHDataset2014/OfForestAndMen/)
	3. [Elephants Dream](http://ftp.itec.aau.at/datasets/DASHDataset2014/ElephantsDream/)
	4. Note: the **2s**, **4s** and **6s** folders should be placed in the roots of ``BigBuckBunny``, ``OfForestAndMen`` and ``ElephantsDream`` (e.g., ``BigBuckBunny/2s/...``, ``BigBuckBunny/4s/...`` and ``BigBuckBunny/6s/...``)
15. Navigate to ``root/paper/paper-utilities``
16. Convert the 9 MPDs with *simple* in the name using the ``Convert_to_BBA2.py`` script
	1. ``python Convert_to_BBA2.py /full/path/to/simple.mpd`` produces MPDs compatible with our BBA2, BBA2-CL and BBA2-CLDouble ABR algorithms
	2. E.g., ``python Convert_to_BBA2.py root/datasets/BigBuckBunny/2s/BigBuckBunny_2s_simple_2014_05_09.mpd``
17. Navigate to ``root/datasets`` and execute ``pwd`` to retrieve the full path to this folder, copy this in your clipboard
18. Open ``root/vegvisir/paper_experiment_full.json`` and paste the copied path in the ``settings > www_dir`` JSON key (bottom of file)  

**Run the experiment**

19. Navigate to ``root/vegvisir``, make sure the virtual environment is enabled
20. Execute ``python -m vegvisir run paper_experiment_full.json``
	1. Vegvisir will show the experiment progress in the console
	2. If any errors occur, recheck the above steps for any mistakes
21. After completion, the results of all testcases can be found in ``root/vegvisir/logs/cross_layer_paper/{datetime of run}``
	1. The folder structure will be the same as ``paper-logs`` explained in the section above.

**Convenience script**  
Checking every testcase folder individually is cumbersome and it makes comparing results difficult. As such we have provided a small convenience HTML page that autoloads the graphs produced by the above experiment and displays them on a grid.  
*Note: This script only displays graphs for the above mentioned datasets. If you want to display other datasets/test setups, please change the script accordingly.*

22. Copy the ``root/paper/paper-utilities/visualize_ouput.html`` file to ``root/vegvisir/logs/``
23. Open and edit the variable on **line 86** to represent the correct folder prefix as explained in instruction 19
24. Navigate to ``root/vegvisir/logs/`` and perform ``python -m http.server``
25. Open a web browser and navigate to [http://127.0.0.1:8000/visualize_ouput.html](http://127.0.0.1:8000/visualize_ouput.html)
