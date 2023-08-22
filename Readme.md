# Dev

[![Go Reference](https://pkg.go.dev/badge/github.com/matthewmueller/dev.svg)](https://pkg.go.dev/github.com/matthewmueller/dev)

Personal CLI for development.

![dev](https://github.com/matthewmueller/dev/assets/170299/7ef4e9e5-a529-4030-a480-730a2e53c32b)

## Features

- Serve directories with live-reload support
- Watch for changes and re-execute a command
- Finds the next available port

## Install

```sh
go install github.com/matthewmueller/dev@latest
dev -h
```

## Contributing

First, clone the repo:

```sh
git clone https://github.com/matthewmueller/dev
cd dev
```

Next, install dependencies:

```sh
go mod tidy
```

Finally, try running the tests:

```sh
go test ./...
```

## License

MIT
