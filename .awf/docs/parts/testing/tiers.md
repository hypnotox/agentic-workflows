## Tiers

awf has a single tier: `./x gate` runs everything, and `./x gate full` runs the
identical steps — the `full` argument is accepted only so the rendered pre-push hook
payload (which invokes `./x gate full`) works unchanged. There is no slower, fuller
tier to reach for; the whole gate is fast enough to run before every commit.
