# Yongo
Simple docker monitoring tool.

# Goals
Since i started to do freelance, I have a lot of containers on diffrernt machines that should be on, so I needed some monitoring tool.
Quick research showed, that all monitoring tools are overkill for me, so I decided to make my own.


# Proof of concept
```
ssh -N -L /tmp/local_socket:/var/run/docker.sock [username]@[ip] -p [PORT]
watch curl --unix-socket /tmp/local_socket http://localhost/containers/json 
```

# Planned features:
## Easy install
Make a simple curl requsts to install monitoring to a system.
## Only nessasary info
"Is continainer running? What are in logs?" - this is 2 questions i want to get answers to.
## No HTTP server
User ssh-socket to get the info to the hub.
## Simple alert system
Telegram bot as an alert system



