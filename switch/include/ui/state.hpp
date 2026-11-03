#pragma once
#include "types.hpp"
#include <algorithm>
namespace bird {
// Selection is global across platforms/playlists, keyed by server ROM id.
class Selection {
  std::vector<Rom> items_;

public:
  bool contains(int id) const {
    return std::any_of(items_.begin(), items_.end(),
                       [id](const Rom &r) { return r.id == id; });
  }
  void toggle(const Rom &r) {
    auto i = std::find_if(items_.begin(), items_.end(),
                          [&](const Rom &v) { return v.id == r.id; });
    if (i == items_.end())
      items_.push_back(r);
    else
      items_.erase(i);
  }
  void all(const std::vector<Rom> &rows) {
    bool full = !rows.empty() &&
                std::all_of(rows.begin(), rows.end(),
                            [&](const Rom &r) { return contains(r.id); });
    for (const auto &r : rows)
      if (contains(r.id) == full)
        toggle(r);
  }
  // A select-all operation that had to fetch pages always adds; it cannot
  // interpret a previously incomplete view as an instruction to deselect.
  void select_all(const std::vector<Rom> &rows) {
    for (const auto &r : rows)
      if (!contains(r.id))
        items_.push_back(r);
  }
  const std::vector<Rom> &items() const { return items_; }
  void clear() { items_.clear(); }
};
inline int move_focus(int focus, int delta, int count) {
  return count ? std::clamp(focus + delta, 0, count - 1) : 0;
}
inline int grid_top(int focus, int top) {
  int row = focus / 4;
  return row < top ? row : row > top + 1 ? row - 1 : top;
}
} // namespace bird
