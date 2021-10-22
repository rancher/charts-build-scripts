## Experimental: Caching

If you specify `export USE_CACHE=1` before running the scripts, a cache will be used that is located at `.charts-build-scripts/.cache`. This cache is only used on `make prepare`, `make patch`, and `make charts`; it is intentionally disabled on `make validate`.

This cache will be used to store references to anything that is pulled into the scripts (e.g. anything defined via UpstreamOptions, such as your upstream charts). If used, the speed of the above three commands may dramatically increase since it is no longer relying on making a network call to pull in your charts from the given cached upstream.

However, caching is only implemented for UpstreamOptions that point to a GitHub Repository at a particular commit, since that is an immutable reference (e.g. any amends to that commit would result in a brand-new commit hash).

If you would like to clean up your cache, either delete the `.charts-build-scripts/.cache` directory or run `make clean-cache`.