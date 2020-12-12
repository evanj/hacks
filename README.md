# Hacks: Small useful things

I often write little throwaway tools when I am working. Occasionally, I write the same tool more than once, at which point I try to turn it into a more serious thing. This is my collection of them. These are for me, but I'm making this public and open source in case they are helpful to others.


## timeparse: parse a time into local, UTC, and unix times

Attempts to guess what format a time is in, and converts it to local time, UTC, and unix timestamps. Example:

```
$ go run ./timeparse 'Sat Dec 12 13:27:44 EST 2020'
Sat Dec 12 13:27:44 EST 2020 (unix_date)
  LOCAL: 2020-12-12T13:27:44-05:00  UTC: 2020-12-12T18:27:44Z  UNIX EPOCH: 1607797664
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
