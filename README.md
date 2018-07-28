# goprocess

`goprocess` provides a simple channel-based API for processes.

## Installation

    go get github.com/NIPE-SYSTEMS/goprocess

Build status: [![Travis CI](https://api.travis-ci.org/NIPE-SYSTEMS/goprocess.svg?branch=master)](https://travis-ci.org/NIPE-SYSTEMS/goprocess)

## Usage

Read the docs: [![GoDoc](https://godoc.org/github.com/NIPE-SYSTEMS/goprocess?status.svg)](https://godoc.org/github.com/NIPE-SYSTEMS/goprocess)

Create a new process:

```go
stdin := make(chan []byte)
signals := make(chan os.Signal)
stdout, stdin, err := NewProcess([]string{"cat"}, stdin, signals)
if err != nil {
    // handle error
}
```

Send SIGINT signal and stop the process:

```go
signals <- syscall.SIGINT
close(stdin)
close(signals)
```

Read the docs for detailed informations of the usage.

## License

MIT