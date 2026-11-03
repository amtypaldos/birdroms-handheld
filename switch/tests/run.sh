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
c++ -std=c++17 -Wall -Wextra -Iinclude tests/folders_test.cpp source/config/folders.cpp -o tests/build/folders_test
tests/build/folders_test
c++ -std=c++17 -Wall -Wextra -Iinclude tests/seed_config_test.cpp source/config/storage.cpp -o tests/build/seed_config_test
tests/build/seed_config_test
c++ -std=c++17 -Wall -Wextra -Iinclude tests/update_policy_test.cpp -o tests/build/update_policy_test
tests/build/update_policy_test
sh tests/update_tests.sh
sh tests/update_http_tests.sh
python3 tests/package_test.py
echo "All Switch host test suites passed"
