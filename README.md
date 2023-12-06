# Welcome to Siege

## Overview
Siege is a powerful tool designed to generate API documentation and insights directly from network traffic, specifically tailored for high performing teams. By running this listener alongside your server or container, you gain the ability to infer API schemas and track essential API-level metrics without compromising data privacy or security.

## Key Features:
- API Schema Inference: Automatically deduce the structure of your API traffic.
- Metric Tracking: Monitor key metrics at the API level.
- Data Privacy: Request or response bodies are never sent to our servers.
- Data Sanitization: Only sanitized data, including schemas and metrics, are transmitted.

# Installation
The Siege Listener can be installed on an system you choose, whether it's in the cloud or on premise, as long as the traffic is unencrypted. For instance, if you have Nginx installed, Siege Listener should be installed behind Nginx, where traffic is decrypted.

## Get your API key
Request an API key [here](https://siegeai.com/#contact)

## Docker
Docker is the fastest and simplest way to install Siege Listener.

#### Install via Docker
Please go to [Docker website](https://docs.docker.com/engine/install/) to install Docker CE/EE. Choose the right installation guide for your system.

#### Install Libpcap dependency
Ubuntu: `sudo apt-get install libpcap-dev` \
Centos/Redhat: `sudo yum install libpcap-devel` \
Mac: `brew install libpcap`

#### Run
`docker run -d --network=host -e SIEGE_APIKEY={YOUR SIEGE API KEY} -e SIEGE_FILTER="tcp and port 80" -e SIEGE_LOG=debug public.ecr.aws/v1v0p1n9/siegelistener:latest`

## Install via binary directly
You may opt for this method if you want to listen to traffic on the host machine.

#### Install Libpcap dependency
Ubuntu: `sudo apt-get install libpcap-dev` \
Centos/Redhat: `sudo yum install libpcap-devel` \
Mac: `brew install libpcap`

#### Create .env file
Configuring Siege is straightforward and can be done by creating a `.env` file in the directory of your choice. like such `vi .env`.

#### Required Environment Variables:
- `SIEGE_APIKEY`: Unique key that identifies your in our multi-tenet infra.
- `SIEGE_DEVICE`: Specifies the network device to listen on. Use `lo` for Ubuntu, `eth0` for Centos, `lo0` for Mac.
- `SIEGE_FILTER`: Defines the packet filters for analysis. We recommend specific TCP filters like `tcp and port 80` or `tcp` for all ports.

#### Optional Configuration:
GOMEMLIMIT: Set a memory limit that suits your environment for optimal performance.

#### Download binary
Download the latest and greatest binary directly from the [Releases page](https://github.com/siegeai/siegelistener/releases)

#### Modify permission
Make sure the binary is runnable `chmod +x ./siegelistener`

#### Run binary
Run binary in background: `./siegelistener &`

## Access your Siege Dashboard
Go to https://dashboard.siegeai.com/ to see your API docs generated live along with endpoint level metrics!

## Support and Contribution
For support, feature requests, or to contribute to the project, please visit our GitHub Issues page.
