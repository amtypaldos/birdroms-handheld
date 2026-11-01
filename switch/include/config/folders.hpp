#pragma once
#include <string>
#include <vector>
namespace bird {
struct FolderPage {
  std::string path;
  std::vector<std::string> directories;
};
std::string folder_start(const std::string &root, const std::string &preferred);
std::string folder_parent(const std::string &root, const std::string &path);
std::string folder_child(const std::string &root, const std::string &path,
                         const std::string &name);
FolderPage list_folders(const std::string &root, const std::string &path);
} // namespace bird
