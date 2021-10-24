module github.com/unhandled-exception/tgproxy-go

go 1.17

require (
	github.com/go-chi/httplog v0.2.0
	internal/pkg/channels v1.0.0
	internal/pkg/httpapi v1.0.0
)

require (
	github.com/go-chi/chi v1.5.4 // indirect
	github.com/go-chi/chi/v5 v5.0.4 // indirect
	github.com/go-chi/render v1.0.1 // indirect
	github.com/go-resty/resty/v2 v2.6.0 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/rotisserie/eris v0.5.1 // indirect
	github.com/rs/zerolog v1.25.0 // indirect
	golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4 // indirect
)

replace internal/pkg/httpapi => ./internal/pkg/httpapi

replace internal/pkg/channels => ./internal/pkg/channels
