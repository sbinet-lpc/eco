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

## Example

```
# CO2 Evolution

Last Updated: 2019-10-18 08:06:05 (UTC)
Stats
missions:     591 (executed)
missions:      57 (planned)
missions:     648 (all)
time period: 2018-10-02 -> 2019-10-18

## Transport (executed, planned, all)

bike           0     0     0
tramway        3     0     3
train        123     8   131
bus            6     0     6
passenger     10     0    10
car          367    34   401
plane         82    15    97

## Distances (executed, planned, all)

bike              0 km        0 km        0 km
tramway          15 km        0 km       15 km
train         81093 km     5844 km    86937 km
bus            1685 km        0 km     1685 km
passenger      2981 km        0 km     2981 km
car          148395 km    16149 km   164544 km
plane        638511 km   159958 km   798469 km
```

![co2](https://github.com/sbinet-lpc/eco/raw/master/testdata/co2.png)
