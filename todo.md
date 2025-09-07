# TODO

## Core-API Permissions

## Implement permissions on core-api service 
    - permissions levels:
        - admin-only
        - admin and users only
        - anyone

## JWT Authentication system
    - system should be validated on following services
        - core-api
        - vue
        - 2 tokens
            - bearer token
            - server-only token (can't remember name - type of token that users can't interact with (browser token?))

## Password Hashing
    - passwords should not be stored in plaintext in the database
    - hashing should probably be done with a library rather than manually implementing an algorithm

## Registration System
    - email verification

### Entire Frontend is liable to change!
Ideally this changes to Nuxt sooner rather than later.
Vite and pinia are working concepts, but I don't think these are meant to be used in production
Even if this whole project is just a 'playground' or 'proof of concept',
there is no purpose in 'proving' that dev tools work if you can't deploy them.

Vite is a development server and I don't know what pinia really is. I think Nuxt allows for SSR which
makes permissions easy for my old boomer brain.

Even so, I am going to develop _something_ with just vite/pinia. Something aside from the weather app.
I may as well create something to give me reason to rewrite the whole frontend. Even if it would be _ideal_ to
use Nuxt instead of Vite, there is no sense in recreating the frontend if the frontend has no purpose to begin with.

So projects are needed. Projects should be data-focused to take advantage and test Go's concurrency.

Ideas:
- Weather Tracker
  - write a cron job to fetch the weather report once every 6 hours
  - graph over time
  - display graph on frontend
  - ensure all data is normalized (ie: don't let raw Windy.com data in - MUST use a common data pattern in case Windy.com drops support or breaks)
  - settle on a data structure - id, date, time, city, temp, sun, cloud, windspeed,...
  - PROS:
    - builds up data fast
    - passive
    - data is easy to get
    - concurrent processing could be used here
  - CONS:
    - uptime is a concern
    - not much use (aside from graphing over time and averaging - but what is the purpose of this aside from visual interest?)
- Chatbox
  - websocket chat box with other authenticated users
  - PROS:
    - is cooool
  - CONS:
    - no users
    - if users, will I need to moderate? (yes-probably)
    - I don't think concurrent processing could be showcased here?
- Twitch Chatbot?
  - could be interesting?
- Twitter integration
  - logging user behavior and tweets over time
  - PROS
    - probably lots of data
  - CONS
    - creepy
    - what would I do with this data?  Tell when someone is likely to tweet?
