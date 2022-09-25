# CES27

To run the process

On a terminal, run:
```
go run SharedResource.go
```

On two or more other terminals, run, where processId is one of yours selected processID:

```
go run Process.go processID_1 :(processID_1+10001) :(processID_2+10001) :(processID_3+10001)
```