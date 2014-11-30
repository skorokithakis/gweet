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
-----

To use Gweet, all you need to do is POST a few values to your server, to a specific `key`. The latter can be
whatever you want, but a UUID is recommended, to avoid conflicts.
The API endpoints are:

* To submit some values, POST `/stream/{key}/` with the values in the query string or in the form-encoded body.
* To read the last posted values, GET `/stream/{key}/`.
* You can get only the `N` latest values with a GET to `/stream/{key}/?latest=N`.
* You can start a permanently-connected endpoint that will stream values to you as they arrive. To do that, just
request `/stream/{key}/?streaming=1`, and the data will be sent to you in a chunked encoding as soon as they arrive.

Simple as that.


Demo
----

If you want to play around with Gweet a bit, there's a hosted demo at:

http://gweetapp.appspot.com/

Feel free to use it, within reason. No guarantees for persistence or continued service.
