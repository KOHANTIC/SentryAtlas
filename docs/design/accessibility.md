# Accessibility checks

What the new site (M4) must pass, written as checks a test or a reviewer can run. From round 2 (#30); the states they apply to are drawn in [`round-2/states.html`](round-2/states.html).

1. **Text contrast 4.5:1, marks and rules 3:1**, in both themes, against the ground each sits on. The token file (#31) carries a test that computes every pair. Round 2's palette passes in both themes: text 6.2:1 or more in light and 7.3:1 in dark, marks 4.8:1 and 5.9:1. The coastline is decorative, at about 2:1.
2. **Targets at least 24 by 24 px** for every control, including the sheet's buttons and map markers when tapped. Aim for 44 px on phones.
3. **No drag-only control.** The phone sheet steps by its More, Less and Map buttons. A time slider (#42) has buttons or a field beside it.
4. **Focus order follows the page:** skip link, header, key and filters, the list, then the open event. Opening an event moves focus to its title; closing it returns focus to its row.
5. **Focus is always visible:** a 3 px ring in the action color, offset from the control, in both themes.
6. **Severity is never shown by color alone:** the mark's fill (hollow, half, full, haloed) and the printed word.
7. **Reduced motion is respected:** with it set, the map jumps instead of flying, and the sheet moves without easing.
8. **The list is the map's twin:** everything on the map is reachable as text, with the same filters (#37). It is also the view when the browser cannot draw the map (no WebGL).

## States the build must draw

| State | When | What it says |
|---|---|---|
| Loading | Before the snapshot arrives | Land first, then each source with its count as it arrives |
| Empty | No event matches the view | That nothing happened, when the sources last looked, and one change likely to show something |
| Partial data | A source's last poll failed | "NWS has not answered for 20 minutes; showing its last update." Normal, not an error |
| Error | Nothing could be loaded | What happened and when it tries again; the land stays |
| Ended event | A link to an event no longer in its feed | It ended, and why its details are gone (the detail endpoint answers 404) |
| No WebGL | The browser cannot draw the map | The list, with everything else working |
