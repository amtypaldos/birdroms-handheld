#pragma once
#include "types.hpp"
namespace bird {
Config load_config(const std::string &);
Config with_seed_server(Config, const std::string &seed_path);
void save_config(const std::string &, const Config &);
// UI persistence boundary: refuse any write before pairing completes.
void save_paired_config(const std::string &, const Config &);
std::string filename(const Rom &);
std::string platform_directory(const Config &, const std::string &);
std::string destination(const Config &, const Rom &);
bool existing(const Config &, const Rom &);
} // namespace bird
