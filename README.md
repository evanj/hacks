# Hacks: Small useful things

I often write little throwaway tools when I am working. Occasionally, I write the same tool more than once, at which point I try to turn it into a more serious thing. This is my collection of them. These are for me, but I'm making this public and open source in case they are helpful to others.


## timeparse: parse a time into local, UTC, and unix times

Attempts to guess what format a time is in, and converts it to local time, UTC, and unix timestamps. Example:

```
$ go run ./timeparse 'Sat Dec 12 13:27:44 EST 2020'
Sat Dec 12 13:27:44 EST 2020 (unix_date)
  LOCAL: 2020-12-12T13:27:44-05:00  UTC: 2020-12-12T18:27:44Z  UNIX EPOCH: 1607797664
```


## makeflaky: pause a process to make tests flaky

Many tests are flaky because they depend on real time, or other scheduling order between threads. This tool sends SIGSTOP/SIGCONT signals to temporarily pause and resume a process, which causes tests to run much slower, like they do on slow CI servers. Use `--stopPeriod` to adjust the length of the pause. Example:

```
$ go run makeflaky.go --stopPeriod=20ms 6034
2020/12/22 16:55:04 slowing down pid=6034; runPeriod=10ms; stopPeriod=20ms (33.3% duty cycle)...
2020/12/22 16:55:49 sent 1307 signals
```


## protodecode: decode protocol buffer bytes without a schema

This is useful for debugging raw data that contains a protocol buffer, but you aren't sure which. It can also partially decode corrupt or invalid protocol buffer messages. This makes it useful for debugging! Example:

```$ go run ./protodecode --nested=4 out
bytes 0-11: field=1 type=0 (varint) uint=9223372036854775808
bytes 11-25: field=2 type=2 (length-delimited) len=12 str="Héllo 🌎!" hex=48c3a96c6c6f20f09f8c8e21
bytes 25-39: field=4 type=2 (length-delimited) len=12 nested message
  bytes 27-33: field=1 type=0 (varint) uint=1607863096
  bytes 33-39: field=2 type=0 (varint) uint=437553000
```


## postgrestmp: start a temporary postgres shell

Creates a new Postgres database in a temporary directory, then runs the psql command line utility to connect to it. When psql exits, the database is deleted. Example:

```
$ go run ./postgrestmp 
initializing temporary postgres database in /var/folders/s_/cmjk7jmx445cbktl7q_p2z0r0000gn/T/postgrestmp_707622637 ...

[... postgres output omitted ...]

starting psql ...
2020-12-12 13:56:45.520 EST [4446] LOG:  database system was shut down at 2020-12-12 13:56:45 EST
2020-12-12 13:56:45.523 EST [4445] LOG:  database system is ready to accept connections
psql (13.1)
Type "help" for help.

postgres=# create table hello (id integer, value text);
CREATE TABLE
postgres=# insert into hello values (1, 'a'), (1, 'b');
INSERT 0 2
postgres=# select id, count(*) from hello group by id;
 id | count 
----+-------
  1 |     2
(1 row)

postgres=# \q
2020-12-12 13:57:24.747 EST [4445] LOG:  database system is shut down
```

## httpping: Time HTTP(s) requests

Executes a sequence of HTTP GET requests to a URL and reports some average statistics about the requests. It also logs the individual requests which are slower than a given threshold. I think I used this to get some average latency numbers, and to check for slow request outliers.

```
$ go run ./httpping https://www.google.com/
2020/12/12 14:04:40 pinging https://www.google.com/ ...
2020/12/12 14:04:40 slow request duration=226.668295ms; start=2020-12-12 14:04:40.379114 -0500 EST m=+0.000624301; end=2020-12-12 14:04:40.605781 -0500 EST m=+0.227292596
2020/12/12 14:04:45 slow request duration=120.824503ms; start=2020-12-12 14:04:45.434547 -0500 EST m=+5.056064727; end=2020-12-12 14:04:45.555372 -0500 EST m=+5.176889230
2020/12/12 14:04:55 204 requests in 15.007340924s = 13.59 req/sec rate; slowest=226.668295ms ; total 2857515 body bytes = 14007.4 bytes/req
```