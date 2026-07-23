# Void Arena custom games

Void Arena supports browser-created custom matches with selectable game mode, map, visibility, optional password, bot count, bot skill, round duration, and server title.

## Visibility

- **Public:** Listed in the in-game server browser and joinable by anyone unless password protected.
- **Password protected:** Listed with a locked indicator. Players are prompted for the password before joining.
- **Unlisted:** Not displayed in the server browser. Players can join through the shared match link or server reference.

## Bots

Custom games can start with the requested number of original Sauerbraten AI bots. Bots are simulated by an assigned connected client and ownership is reassigned when possible if that client leaves.

## Rotation and synchronization

Configured server rotations avoid selecting the current map when another map is available. Relay queues are cleared safely, bot-owned packets are routed separately, and damage is rejected until both attacker and target have confirmed their current spawn state.
