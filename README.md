check_database
==============

`check_database` is a small tool which runs SQL statements against databases,
then checking the result against limits.

building
--------

To build `check_database`, run `go build`.

configuration
-------------

Copy `check_database.conf.example` to `check_database.conf` and adjust the
settings to your needs.

running
-------

Run `check_database --help` to get the basic syntax of the program.

To list all configured databases run `check_database list databases`.

To list all queries run `check_database list queries`.

To run a query on a database, call `check_database run <queryname> on <database>`.

When the query requires parameters, it is possible to add them after the command:

```
check_database run foo on localhost --param1 foobar --param2 baz
```

To see which parameters are available for a specific query, run the following
command:

```
check_database run foo on localhost --help
Usage of check_database run foo on localhost:
  -return string
		parameter for return
	-trigger string
		Which trigger should be checked?
```
