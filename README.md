# hroll

*MEGA WIP*

This is my attempt at creating a load balancing socks5 proxy. It
should rotate between socks5 proxies depending on which is the
best for the target.

It uses haproxy for the proxying and this program to manage
the haproxy configuration.

This is my first time using haproxy, so this might not make any
sense.

This was mostly made because [nadoo/glider](https://github.com/nadoo/glider)
did not support advanced healthchecks and also that it didn't
allow live reloading of the config.


## SETUP

Create a folder under this called `haproxy-stuff` with the following layout:

```
haproxy-stuff
├── bin
├── conf
├── logs
├── sockets
└── txns
```

Where you place the file from `examples/haproxy.conf` in the conf directory



## TODO

- It has latency checking and stuff to.
- Use server names for info about last uptime and such
- Use viper or something else for conf

## See also

- [glider](https://github.com/nadoo/glider)
