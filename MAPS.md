# Void Arena map pack

This build packages 20 maps from the stock Cube 2: Sauerbraten multiplayer catalog used by the pinned r6481 game assets. The build rejects unknown names and fails if any requested `.ogz` map cannot actually be packaged, so a release cannot silently ship a menu entry without its map data.

## Free For All / Instagib / Efficiency

- academy
- complex
- dust2
- frostbyte
- gorge
- hades
- injustice
- lostinspace
- metl3
- metl4
- oasis
- paradigm
- phosgene
- ruby
- stronghold
- suburb
- turbine

## Capture

- dust2
- forge
- frostbyte
- gorge
- hades
- lostinspace
- paradigm
- reissen
- ruby
- stronghold
- suburb
- venice

## Capture the Flag

- dust2
- forge
- reissen
- stronghold
- suburb

The native in-game map menus are generated from these same lists during the asset build. `assets/dist/packaged-maps.json` records the exact manifest included in a release.
