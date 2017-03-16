Gweet
=====
Gweet is a ridiculously simple message queue for use with internet-connected devices. Post and read messages to
arbitrary keys with ease. You can find more information about it on my blog:

http://www.stavros.io/posts/messaging-for-your-things/


Installation
------------

To install, run

```
go build github.com/skorokithakis/gweet
```

and copy the resulting `gweet` binary somewhere, anywhere. You're ready, just run it and your server is operational.

You can also download a pre-built binary from the Github releases page.


Features
--------

Gweet supports simple pub/sub, reading old messages, and real-time
notifications. It also supports publishing messages to a channel without knowing
its key (and without being able to read from it), by publishing to the SHA2-256
hash of the key. Obviously, since SHA2 is brute-forceable, make sure to choose
long key names.


Usage
-----

To use Gweet, all you need to do is POST a few values to your server, to a specific `key`. The latter can be
whatever you want, but a UUID is recommended, to avoid conflicts.
The API endpoints are:

* To submit some values, POST `/stream/{key}/` with the values in the query
  string or in the form-encoded body.
* To read the last posted values, GET `/stream/{key}/`.
* You can get only the `N` latest values with a GET to `/stream/{key}/?latest=N`.
* You can start a permanently-connected endpoint that will stream values to you
  as they arrive. To do that, just request `/stream/{key}/?streaming=1`, and the
  data will be sent to you in a chunked encoding as soon as they arrive.
* To allow clients to submit a value to a channel without being able to *read*
  from that channel, i.e. to a push-only channel, calculate the SHA2-256 of the
  key and give that to the client. The client can then POST to `/push/{sha2}/`.
  For example, to publish to the channel `/stream/hello/`, a client can POST to
  `/push/2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824`.

Simple as that.


Demo
----

If you want to play around with Gweet a bit, there's a hosted demo at:

https://queue.stavros.io/

Feel free to use it, within reason. No guarantees for persistence or continued service.
