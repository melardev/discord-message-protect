# Introduction

This Discord bot will expose a command to protect messages from automated systems. It will require from the users
to pass the challenge (manual interaction + optionally a captcha if configured).
This Discord bot was created as a learning experience, although I try to provide it at least as a template project to
use,
It is not meant to be a sophisticated distributed enterprise application with complex features,
just a simple, yet usable Bot that is run from one single server.

# How to

TODO: Complete this section with screenshots

1. Create a bot exactly as indicated in this Youtube Video<br>
2. Now we need a server to run our bot 24/7, I choose DigitalOcean cloud provider to rent a server where I will run my
bot:
3. Access our server.<br>
4. The bot is written in Go, download Go, the bot and compile it to an executable file<br>
5. Make sure the bot is now compiled and ready to be used<br>
6. If we just run the bot now, as soon as we disconnect from our server, the bot will stop running,
we need to run it in a detached screen session, so let's create such session
7. now you can list the sessions we have in our server<br>
8. we see our session is created, let's attach to it<br>
9. now we can run our bot<br>
10. to detach from the session hold Ctrl while you press in sequence `A` and `D` in the keyboard.<br>
11. Make sure your bot is still running<br>

# Features

- Protect messages requiring user to click to reveal the message
- Protect with captcha
- Pollute messages with unique identifiers.

# Flow

1. Author creates secret
2. Bot "memorizes" the secret
3. Creates interact button
4. User Interacts
5. User gets link
6. User visits link + resolves challenge
7. User gets secret

# Pollution

The bot can "tint" or "pollute" messages with a unique identifier per user.
This can uncover which user is exfiltrating messages to other servers.
It is possible because the protected messages are sent to each user privately, so it is possible
to send each one a message slightly different. It is a well known technique used in companies to
detect rogue employees, for example Tesla used it to uncover the employee selling the news to the
media ([Full story](https://www.ndtv.com/world-news/elon-musk-explains-how-tesla-caught-employee-leaking-data-3433802)).
The draw-down is rogue users can program their bots to detect these ids and remove them before leaking them, this is
why multiple pollution strategies are implemented, some strategies are easier to detect and neutralise, some are harder
and will force the rogue user's bot to crop a big chunk of the message leaving it meaningless.

The pollution procedure is, legitimate user creates a protected message, the pollution mechanism adds some unique
identifiers per
user,
the rogue user's bot exfiltrates the polluted message with the indicators, you go to the server where the messages are
being exfiltrated,
retrieve the pollution indicators, those identify a single and unique message tied to a specific user, you check the
logs
to see which user was given those indicators, once you find it, you know who is the rogue user.



# TODO

- Filesystem/Database based implementations, for now only in-memory implementations are provided, if the app is
  restarted,
  protected messages would not be recoverable.
- Graceful exit, we must have a mean to exit all app components(secret manager, session manager, etc) gracefully without
  corrupting the work they are engaged in at the time.
- Protect by role, ability to use the protect command may be restricted by role.
- Edit the secrets
- Delete the secrets.
- Delete interaction button after user clicked on it
- send the protected message if user has recently passed the challenge, no need to make him/her click it again
- Maintenance tasks (remove all protected messages, uncover all protected messages, etc.)
- Rate Limit
- Implement the Random IndicatorPosition for all pollution strategies.
- provide ability to choose pollution method via command argument, overriding the current's pollution config settings
- Clean up the architecture, there are some relations or assumptions that should not exist, like http server knowing
  about the captcha html code, that should be left to the specific captcha impl as captcha usage differs.