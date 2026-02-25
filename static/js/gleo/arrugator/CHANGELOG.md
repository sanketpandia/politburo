
# v1.3.2 2024-06-26

-	Tweak failure mode of epsilon growth in forced splits, to minimize artefacts

# v1.3.1 2024-06-26

-	Fixed typo

# v1.3.0 2024-06-26

-	Added `force()` method to allow overcoming some artefacts
-	Added detection of epsilon stall
-	Removed `preSplitGrid`

# v1.2.1 2022-11-30

-	Added `3rd-party/tinyqueue.mjs` to NPM release

# v1.2.0 2022-11-30

-	Added `preSplitGrid` exported function
-	Prevent queueing segments which would raise epsilon

# v1.1.0 2022-09-28

-	Added setter/getter for the `epsilon`

# v1.0.1 2021-03-09

-   Include a copy of tinyqueue in the npm distrib.

# v1.0.0 2021-03-09

-   Initial release
-   API shape is considered stable - implementation of flat arrays as per bug #1 would mean a major version change (to 2.0.0)
