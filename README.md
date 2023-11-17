# Welcome to Siege

## Overview
Siege is a powerful tool designed to generate API documentation and insights directly from network traffic, specifically tailored for high performance engineering teams. By running this listener alongside your server, you gain the ability to infer API schemas and track essential API-level metrics without compromising data privacy or security.

## Key Features:
- API Schema Inference: Automatically deduce the structure of your API traffic.
- Metric Tracking: Monitor key metrics at the API level.
- Data Privacy: Request or response bodies are never sent to our servers.
- Data Sanitization: Only sanitized data, including schemas and metrics, are transmitted.

## Configuration
Configuring Siege is straightforward and can be done through environment variables or a `.env` file.

## Required Environment Variables:
SIEGE_APIKEY: Request an API key [here](https://siegeai.com/#contact).
SIEGE_DEVICE: Specifies the network device to listen on. Use "lo" for Linux and "lo0" for Mac.
SIEGE_FILTER: Defines the packet filters for analysis. We recommend specific TCP filters like "tcp and port 80".

## Optional Configuration:
GOMEMLIMIT: Set a memory limit that suits your environment for optimal performance.

## Getting Started
1. Run the Docker image
2. Set Up Environment Variables: Refer to the configuration section above.
3. Run the Tool: Start the program and begin gaining insights from your network traffic.

## Support and Contribution
For support, feature requests, or to contribute to the project, please visit our GitHub Issues page.
