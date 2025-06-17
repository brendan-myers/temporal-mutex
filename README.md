# Distributed Lock using Temporal

> [!CAUTION]
> This should not be used by anyone, ever.

## What is it?

A distributed lock built using Temporal by misusing activity tasks.

Typically, activity workers all perform the same operations/function when they process a specific activity task. e.g. It shouldn't matter *which* activity worker handles a `SendNotification` activity task, all workers should handle the task the same way. Additionally, it shouldn't matter *how many* `SendNotification` activity tasks are scheduled, the Temporal server will queue the tasks (if needed) and assign them to activity workers.

This abomination works in the opposite way; there is only one activity task (the lock), and activity workers (processes requesting the lock) have no requirement to execute the same set of operations once they have fetched the activity task (obtained the lock). Temporal specifics are handled by the mutex library, processes call `mutex.Lock` and `mutex.Unlock` instead of managing workflow/activity details themselves. That said, this is a pretty leaky abstraction, for example, timeouts aren't particularly well hidden from the user.

![Running the example code](./static/example.gif)

## How to run the example

First, run a local Temporal server `temporal server start-dev`, then in (multiple) separate terminals, run;

```sh
go run ./example
```

## Caveats

So, so many. This thing is riddled with gotchas.

## But, why?

Why not 🤷‍♂️