#pragma once
#include "types.hpp"
namespace bird {
constexpr int UpdateReadyExit = 2;
int run_ui(Config, const std::string &config_path,
           const std::string &executable_path);
} // namespace bird
