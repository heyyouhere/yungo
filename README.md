# Yungo
Simple docker monitoring tool.

# Goals
Since i started to do freelance, I have a lot of containers on diffrernt machines that should be on, so I needed some monitoring tool.
Quick research on such tools showed that all of them are are overkill for me, so I decided to make my own.


# Proof of concept
```
ssh -N -L /tmp/local_socket:/var/run/docker.sock [username]@[ip] -p [PORT]
curl --unix-socket /tmp/local_socket http://localhost/containers/json | jq
```

# Planned features:
## Easy install
Copy keys to server. Add user on server to **docker** group. That's it.
## Only nessasary info
"Is continainer running? What's in the logs?" - this is 2 questions i want to get answers to.
## No HTTP server
User ssh-socket to get the info to the hub. Secure and and easy.
## Simple alert system
Telegram bot as an alert system



