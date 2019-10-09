# eco

`eco` is a simple package exposing tools to investigate the carbon footprint of LPC stemming from travel (plane, train, car, bus, ...)

`eco` consists of 3 parts:

- `cmd/eco-ingest`: a command that loads LPC travel (internal) database, analyzes travel missions and uploads cleaned up data to `eco-srv
- `cmd/eco-srv`: a HTTP server that computes statistical data from the cleaned up travel missions
- `cmd/eco-stats`: a simple command that queries `eco-srv` and dumps statistical data on screen.

## Example:

```
$> eco-srv -addr=eco-srv.example.com:80 &
eco-srv: serving "eco-srv.example.com:80"...

$> eco-stats -addr=eco-srv.example.com
eco-stats: querying "eco-srv.example.com"...
eco-stats: missions: 582
eco-stats: === transport ===
eco-stats: bike       0
eco-stats: tramway    2
eco-stats: train      120
eco-stats: bus        6
eco-stats: passenger  10
eco-stats: car        355
eco-stats: plane      89
eco-stats: === distances ===
eco-stats: bike       0 km
eco-stats: tramway    10 km
eco-stats: train      79056 km
eco-stats: bus        1685 km
eco-stats: passenger  2981 km
eco-stats: car        145968 km
eco-stats: plane      722748 km
```


## References

- https://docs.google.com/spreadsheets/d/1WVemrYvkBv3hD_AbIOteL5uRa5cqfBWh/edit#gid=392963105
