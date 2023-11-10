Hello `siegelistener`!

This program can be run alongside a server to gain insights from network traffic.
We infer api schemas and track basic api level metrics.
We don't send request or response bodies to our servers.
We send sanitized versions of the data including schemas and metrics.

Configuration is done via the following env variables (or entries in a .env file)
- SIEGE_APIKEY; this should be set to the api key provided to you by siege
- SIEGE_DEVICE; this determines which device will be listened on. On linux "lo" is good, on mac try "lo0".
- SIEGE_FILTER; this determines which packets will be analyzed. The more specific the better. We expect tcp. "tcp and port 80", or whichever port you expect traffic on is a good choice.

Also consider setting the env var GOMEMLIMIT to something sensible for your use.