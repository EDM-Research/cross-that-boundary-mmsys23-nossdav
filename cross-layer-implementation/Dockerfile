FROM golang:1.19 as godashxl-builder

# Download and install godash cross-layer adaptation
RUN cd /go
COPY ./quic-go ./quic-go
RUN cd quic-go && go mod tidy && go build

# Download and install quic-go cross-layer adaptation
RUN cd /go
COPY ./godash-qlogabr ./godash-qlogabr
RUN cd godash-qlogabr && go mod tidy && go build

# ------------ rest ---------------

FROM martenseemann/quic-network-simulator-endpoint:latest

RUN apt-get update
RUN apt-get install -y git-core python3 python3-pip
RUN apt-get install -y python3-setuptools
#RUN pip install setuptools

RUN git clone https://github.com/itu-p1203/itu-p1203.git && cd /itu-p1203
#RUN cd /itu-p1203 && pip3 install -e . && pip3 install . && python3 -m itu_p1203 examples/mode0.json --accept-notice
RUN cd /itu-p1203 && pip3 install . && python3 -m itu_p1203 examples/mode0.json --accept-notice

COPY --from=godashxl-builder /go/godash-qlogabr/godash /bin/godash
COPY --from=godashxl-builder /go/godash-qlogabr/config/configure.json /configure.json
RUN chmod +x /bin/godash

RUN mkdir -p /logs/files

# copy run script and run it
COPY run_endpoint.sh .
RUN chmod +x run_endpoint.sh
ENTRYPOINT [ "./run_endpoint.sh" ]
