FROM debian:latest
RUN apt update
RUN apt install -y build-essential libpcap-dev

USER root

WORKDIR /root
COPY siegelistener siegelistener
#RUN chmod +x siegelistener

ENV SIEGE_APIKEY sdfklj
ENV SIEGE_SERVER http://host.docker.internal:3000
ENV SIEGE_DEVICE any
ENV SIEGE_FILTER "tcp and port 8080"
ENV SIEGE_LOG debug

CMD ./siegelistener
