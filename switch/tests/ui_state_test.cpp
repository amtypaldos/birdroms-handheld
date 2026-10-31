#include "ui/state.hpp"
#include <cassert>
int main() {
  using namespace bird;
  Rom a;
  a.id = 1;
  Rom b;
  b.id = 2;
  Rom c;
  c.id = 3;
  Selection s;
  s.toggle(c);
  s.all({a, b});
  assert(s.items().size() == 3);
  s.all({a, b});
  assert(s.items().size() == 1 && s.contains(3));
  s.toggle(c);
  assert(s.items().empty());
  s.all({a, a, b});
  assert(s.items().size() == 2);
  s.select_all({a, b, c});
  s.select_all({a, b, c});
  assert(s.items().size() == 3 && s.contains(c.id));
  assert(move_focus(0, -1, 10) == 0);
  assert(move_focus(7, 8, 10) == 9);
  assert(grid_top(8, 0) == 1);
  assert(grid_top(0, 3) == 0);
}
