# Hacks: Small useful things

I often write little throwaway tools when I am working. Occasionally, I write the same tool more than once, at which point I try to turn it into a more serious thing. This is my collection of them. These are for me, but I'm making this public and open source in case they are helpful to others.


## timeparse

This command line utility attempts to guess what format a time is in, and converts it to local time, UTC, and unix timestamps. Example:

```
$ go run ./timeparse 'Sat Dec 12 13:27:44 EST 2020'
Sat Dec 12 13:27:44 EST 2020 (unix_date)
  LOCAL: 2020-12-12T13:27:44-05:00  UTC: 2020-12-12T18:27:44Z  UNIX EPOCH: 1607797664
```

