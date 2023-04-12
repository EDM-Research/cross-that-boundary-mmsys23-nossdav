from vegvisir.environments import webserver, sensors, cl

default_environment = "webserver-basic"
available_environments = {
    "webserver-basic": webserver.WebserverBasic,
    "cross-layer": cl.CrossLayer
}

available_sensors = {
    "timeout": sensors.TimeoutSensor,
    "browser-file-watchdog": sensors.BrowserDownloadWatchdogSensor
}