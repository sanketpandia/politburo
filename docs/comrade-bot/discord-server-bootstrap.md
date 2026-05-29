# Discord server bootstrap and VA Basic Setup

Audience: Discord server admins and maintainers supporting Comrade Bot onboarding.

## What `/initserver` does

`/initserver` claims the current Discord server for a VA with one field: **VA Code / ID**. The command:

- must be run inside the Discord server, not in DMs;
- requires the Discord user to have run `/register` first;
- is registered as an administrator-only slash command;
- creates a minimal VA using the VA Code / ID and binds it to the current Discord server;
- does not collect VA display name, callsign start, callsign end, Airtable, PIREP, route, livery, event, or webhook settings.

After it succeeds, use `/dashboard` to open Vizburo. A desktop browser or desktop-view mode is recommended for setup.

## Finish Basic Setup in Vizburo

Open **VA Admin → VA Setup** at `/dashboard/vaadmin/setup`.

Basic Setup shows the VA Code / ID as read-only and lets admins save:

- optional display name;
- callsign start, for callsigns such as `IFE123`;
- callsign end, for callsigns such as `123VA`;
- enabled Infinite Flight servers for live-flight matching.

At least one callsign start or callsign end is required before live-flight matching is ready. Basic Setup preserves the casing you enter when it saves the callsign rule, while matching remains case-insensitive. The sample callsign tester checks against the currently saved rule.

The enabled servers list is populated from the cached Infinite Flight live sessions. Leave the selection empty to match flights on all available servers. If a previously selected server is no longer available, it is removed from the saved selection the next time Basic Setup is saved.

The setup checklist currently marks bootstrap and flight-matching readiness. Other modules, such as Airtable records, flight logging, notifications, events, and livery mappings, remain optional and are configured from their existing Vizburo pages.

## Developer notes

The bot-facing API endpoint is `POST /api/v1/server/init`. Its request body is only:

```json
{ "va_code": "IFE" }
```

Registration/onboarding calls must send `X-API-Key`, `X-Discord-User-Id`, and `X-Discord-Server-Id`. Do not document or depend on raw VA UUIDs for normal Discord success copy.
