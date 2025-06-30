# Yungo
Simple Docker monitoring tool.

# Goals
Since I started to do freelance work, I have a lot of containers on different machines that should be running, so I needed a monitoring tool.
A quick research on such tools showed that all of them are overkill for me, so I decided to make my own.

# Proof of Concept
```
ssh -N -L /tmp/local_socket:/var/run/docker.sock [username]@[ip] -p [PORT]
curl --unix-socket /tmp/local_socket http://localhost/containers/json | jq
```

# Planned Features:
## Easy Install
Copy keys to the server. Add a user on the server to the **docker** group. That's it.
## Only Necessary Info
"Is the container running? What's in the logs?" - these are the two questions I want to get answers to.
## No HTTP Server
Simple SSH connections. Secure and well-known.
## Simple Alert System
Telegram bot as an alert system.
