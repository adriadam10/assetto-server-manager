# acServer
a rewrite of the assetto corsa server

## goals

* [ ] achieve full compatibility with existing acServer implementation
* [ ] add performance improvements where possible
* [x] build improved server-side plugin system using callbacks
* [ ] improve configuration format (json? yaml?)
* [ ] release in more binary formats (macOS, raspberry pi, etc)
* [ ] build into [server manager](https://github.com/JustaPenguin/assetto-server-manager)
* [x] add support for forcing plugins to be installed (via checksum?)
* [x] sanitise checksums (important, security risk)
* [x] weather change support
* [x] single person on track qualifying
* [ ] server side driver swap stuff
* [ ] race weekend session 'warm up' with no grid implications
* [x] Increase sun update frequency with time multiplier
* [ ] Potentially check for CSP/Sol and allow the server to set sun angles outside of -80, +80

probably more things too.

## message types

* [x] entrylist
* [x] UDP connection init
* [x] UDP association
* [x] checksums
* [x] send current weather
* [x] tyre changes
* [x] damage update
* [x] lap split completed
* [x] lap completed
* [x] client disconnect
* [x] race start
* [x] race over
* [x] session changes - next, restart, etc
* [x] megapacket/status messages
* [x] setup changes, fixed setup
* [x] kick user
* [x] ban user
* [x] block list mode
* [x] DRS zones
* [x] ping averages
* [x] car join process
* [x] admin commands
* [x] broadcast chat
* [x] driver chat
* [x] wind
* [x] BoP
* [x] block list
* [x] client event
* [x] motd
* [x] session openness
* [x] sun angle
* [x] session countdown - dont advance if people are joining
* [x] megapacket - use car distances to prioritise some cars
* [x] mandatory pit

## general functions

* [x] lap monitoring
* [x] session management
* [x] reverse grid races
* [x] weather management
* [x] track conditions/grip
* [x] pickup mode
* [x] locked entrylist
* [x] booking mode
* [ ] connection management
* [x] results saving
* [x] lobby registration
* [ ] client crash register disconnect
* [x] collision info
* [x] graceful server shutdown not using os.Exit()
* [x] check wind speed - we're not using min/max values, and it seems that acServer has them?
* [ ] what happens re results if two drivers share a car in a pickup=1,locked=0 session? do all lap times get shown? our system won't work with that right now.
