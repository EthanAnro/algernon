[pie](https://github.com/natefinch/pie) plugins for Algernon
===========================================================

Plugins must offer `Lua.Code` and `Lua.Help` functions.

See the two plugins examples in this directory for an example of how this can be done.

`Lua.Code` serves Lua code, while `Lua.Help` serves help for the available Lua functions.
The Lua functions can call other plugin functions with the `CallPlugin` function in Algernon.

Plugins can be loaded with the `Plugin` function in Algernon.

#### Example use

First build the executable `plugins/go/go`:

```shell
cd plugins/go
go build
cd ../..
```

Then launch Algernon:

```shell
algernon -e
```

Then at the `lua>` prompt:

```lua
Plugin("plugins/go/go")
add3(4,5)
```

You should get the output `12`.
