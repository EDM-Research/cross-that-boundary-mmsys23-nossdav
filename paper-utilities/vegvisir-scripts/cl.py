import logging
import os
import re
import subprocess
from vegvisir.data import ExperimentPaths
from vegvisir.environments.base_environment import BaseEnvironment
import sys
import numpy as np
import matplotlib.pyplot as plt

class CrossLayer(BaseEnvironment):
	def __init__(self) -> None:
		super().__init__()
		self.environment_name = "cross_layer"
		self.set_QIR_compatibility_testcase("http3")

	def post_run_hook(self, paths: ExperimentPaths):
		# Modified to work with Vegvisir paths
		logger = logging.getLogger("root.Experiment.crosslayerPostHook")

		metrics_log = os.path.join(paths.log_path_client, "metrics_log.txt")
		shaper_metrics_log = os.path.join(paths.log_path_shaper, "shaper_metrics.txt")

		for mode in ["none", "stallprediction", "bba"]:
			output_path = os.path.join(paths.log_path_client, f"viz_{mode}.png")
			try:
				cmd = f"source .venv/bin/activate && python ./util/segmentGraph.py {metrics_log} {shaper_metrics_log} todo-name {output_path} {mode}"
				with open(os.path.join(paths.log_path_client, f"viz_command_{mode}.txt"), "w") as fp:
					fp.write(cmd)
				subprocess.run(cmd, shell=True, executable="/bin/bash")
			except Exception as e:
				logger.error(f"Viz failed {e}")
			
		try:
			pattern = re.compile(r"(?P<index>\d+).m4s.json$")
			file_to_test = None
			highest_index = -1
			for file in os.listdir(os.path.join(paths.log_path_client, "files")):
				# Tested to work for BBB, ED and OFM datasets
				match_obj = pattern.search(file)
				if match_obj:
					found_index = int(match_obj.group("index"))
					if found_index > highest_index:
						highest_index = found_index
						file_to_test = file
			
			if file_to_test:
				try:
					file_path = os.path.join(paths.log_path_client, f"files/{file_to_test}")
					logging.info(f"ITU P.1203 test starting for file [{file_path}]")
					with open(os.path.join(paths.log_path_client, "itu-p1203.json"), "w") as fp:
						subprocess.run(f"source .venv/bin/activate && python -m itu_p1203 --accept-notice {file_path}", shell=True, stdout=fp, executable="/bin/bash")
				except Exception as e:
					logger.error(f"ITU P.1203 failed {e}")

		except Exception as e:
			logger.error(f"ITU P.1203 step failed {e}")
