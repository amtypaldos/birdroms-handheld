#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p tests/build
sh tests/api_tests.sh
c++ -std=c++17 -Wall -Wextra -pthread -Iinclude tests/storage_test.cpp source/config/storage.cpp source/downloads/manager.cpp -o tests/build/storage_test
c++ -std=c++17 -Wall -Wextra -Iinclude tests/config_replace_test.cpp source/config/storage.cpp -Wl,--wrap=rename -o tests/build/config_replace_test
tests/build/config_replace_test
tests/build/storage_test
c++ -std=c++17 -Wall -Wextra -Iinclude tests/ui_state_test.cpp -o tests/build/ui_state_test
tests/build/ui_state_test
c++ -std=c++17 -Wall -Wextra -Iinclude tests/ui_image_bounds_test.cpp -o tests/build/ui_image_bounds_test
tests/build/ui_image_bounds_test
echo "All Switch host test suites passed"
