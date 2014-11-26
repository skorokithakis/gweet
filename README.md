Gweet
=====
Gweet is a ridiculously simple message queue for use with internet-connected devices. Post and read messages to
arbitrary keys with ease.


Installation
------------

To install, run

```
go build github.com/skorokithakis/gweet
```

and copy the resulting `gweet` binary somewhere, anywhere. You're ready, just run it and your server is operational.


Usage
---

To use Gweet, all you need to do is POST a few values to your server, to the `/stream/{key}/` endpoint. `key` can be
whatever you want, but a UUID is recommended, to avoid conflicts. The values can either be in the query string or in
the POST body, encoded as a form.

To read the last posted values, just GET `/stream/{key}/`. Simple as that.
