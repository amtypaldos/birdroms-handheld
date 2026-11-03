#include "config/folders.hpp"
#include <algorithm>
#include <cctype>
#include <filesystem>
#include <stdexcept>
namespace bird {
namespace {
namespace fs = std::filesystem;
std::string normalized(const std::string &value) {
  if (value.empty())
    throw std::runtime_error("Empty directory path");
  const fs::path path(value);
  for (const auto &component : path)
    if (component == ".." || component.string().find('\\') != std::string::npos)
      throw std::runtime_error("Unsafe directory path");
  auto result = path.lexically_normal().generic_string();
  if (result.empty())
    throw std::runtime_error("Invalid directory path");
  if (result.back() != '/')
    result += '/';
  return result;
}
std::string checked(const std::string &root, const std::string &value) {
  const auto base = normalized(root), path = normalized(value);
  if (path.compare(0, base.size(), base) != 0)
    throw std::runtime_error("Directory must stay on the SD card");
  fs::path current;
  for (const auto &component : fs::path(path)) {
    current /= component;
    std::error_code ec;
    auto status = fs::symlink_status(current, ec);
    if (!ec && fs::is_symlink(status))
      throw std::runtime_error("Linked directories are not supported");
  }
  return path;
}
} // namespace
std::string folder_parent(const std::string &root, const std::string &value) {
  auto path = checked(root, value), base = normalized(root);
  if (path == base)
    return base;
  path.pop_back();
  auto parent = path.substr(0, path.find_last_of('/') + 1);
  return checked(root, parent);
}
std::string folder_start(const std::string &root,
                         const std::string &preferred) {
  auto path = checked(root, preferred);
  for (;;) {
    std::error_code ec;
    if (std::filesystem::is_directory(path, ec))
      return path;
    auto parent = folder_parent(root, path);
    if (parent == path)
      throw std::runtime_error("SD card directory is unavailable");
    path = parent;
  }
}
std::string folder_child(const std::string &root, const std::string &path,
                         const std::string &name) {
  if (name.empty() || name == "." || name == ".." ||
      name.find_first_of("/\\:") != std::string::npos)
    throw std::runtime_error("Invalid directory name");
  return checked(root, checked(root, path) + name);
}
FolderPage list_folders(const std::string &root, const std::string &value) {
  FolderPage page;
  page.path = checked(root, value);
  for (const auto &entry : std::filesystem::directory_iterator(page.path)) {
    if (entry.is_symlink() || !entry.is_directory())
      continue;
    auto name = entry.path().filename().string();
    if (name.empty() || name == "." || name == ".." ||
        name.find_first_of("/\\:") != std::string::npos)
      continue;
    if (page.directories.size() >= 4096)
      throw std::runtime_error(
          "Too many subdirectories to display (limit 4096)");
    page.directories.push_back(std::move(name));
  }
  std::sort(page.directories.begin(), page.directories.end(),
            [](const auto &a, const auto &b) {
              return std::lexicographical_compare(
                  a.begin(), a.end(), b.begin(), b.end(),
                  [](unsigned char x, unsigned char y) {
                    return std::tolower(x) < std::tolower(y);
                  });
            });
  return page;
}
} // namespace bird
