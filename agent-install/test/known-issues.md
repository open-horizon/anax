Bugs

- in the end of the script output the script running time displayed twice

Gaps

- Needs to be checked how it runs on ARM architecture/raspbian
- Requires internet connectivity (clone the `shUnit2` and download packages) - cannot run in air-gap environments

Enhancements

- For the test there are two clusters required (or the same cluster can be used as primary and secondary) due to a test case for switching environments - can be probably skipped with a parameter (in a more user friendly manner)
- Add skipping some tests
- Depends on `shUnit2`, for reliability the script should clone a particular commit rather than the latest `master`
- Parameterize more messages for asserts