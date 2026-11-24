import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GroupTreeSelect } from "./GroupTreeSelect";

const groups = [
  { id: "g-1", name: "engineering", displayName: "Engineering" },
  { id: "g-2", name: "frontend", displayName: "Frontend", parentId: "g-1" },
  { id: "g-3", name: "backend", displayName: "Backend", parentId: "g-1" },
];

describe("GroupTreeSelect", () => {
  // 回归：节点只带 key 不带 value 时 antd 会告警
  // 「TreeNode `value` is invalidate: undefined」并拒绝选择父组。
  it("renders selectable nodes without antd TreeNode warnings", async () => {
    const errors: string[] = [];
    const spy = vi.spyOn(console, "error").mockImplementation((msg: unknown) => {
      errors.push(String(msg));
    });

    render(<GroupTreeSelect groups={groups} placeholder="Parent group" />);

    expect(
      errors.filter((line) => line.includes("TreeNode") || line.includes("Same `value` exist")),
    ).toEqual([]);

    spy.mockRestore();
  });

  // 回归：第二轮曾遍历全量 groups，对 excludeIds 中的组取到 undefined 并 push 进
  // children/roots，clean() 读 children 抛 TypeError。编辑任意分组都会命中该路径。
  it("renders when excludeIds contains the current group and its descendants", () => {
    render(
      <GroupTreeSelect groups={groups} excludeIds={["g-1", "g-2"]} placeholder="Parent group" />,
    );

    expect(screen.getByRole("combobox")).toBeInTheDocument();
  });

  it("emits the group id when a group is chosen", async () => {
    const onChange = vi.fn<(value: string | string[]) => void>();
    const user = userEvent.setup();
    const roots = groups.map(({ id, name, displayName }) => ({ id, name, displayName }));

    render(<GroupTreeSelect groups={roots} placeholder="Parent group" onChange={onChange} />);

    await user.click(screen.getByRole("combobox"));
    // label 形态为「displayName (name)」。
    await waitFor(() => expect(screen.getByText("Frontend (frontend)")).toBeTruthy());
    await user.click(screen.getByText("Frontend (frontend)"));

    expect(onChange).toHaveBeenCalled();
    expect(onChange.mock.calls[0][0]).toBe("g-2");
  });

  // 回归：valueKey="name"（application.defaultGroup 口径）时 key=id≠value=name
  // 会触发 rc-tree-select「key or value with TreeNode must be the same」告警。
  it("keeps key equal to value and emits the name when valueKey=name", async () => {
    const errors: string[] = [];
    const spy = vi.spyOn(console, "error").mockImplementation((msg: unknown) => {
      errors.push(String(msg));
    });
    const onChange = vi.fn<(value: string | string[]) => void>();
    const user = userEvent.setup();
    const roots = groups.map(({ id, name, displayName }) => ({ id, name, displayName }));

    render(
      <GroupTreeSelect
        groups={roots}
        valueKey="name"
        placeholder="Default group"
        onChange={onChange}
      />,
    );

    await user.click(screen.getByRole("combobox"));
    await waitFor(() => expect(screen.getByText("Frontend (frontend)")).toBeTruthy());
    await user.click(screen.getByText("Frontend (frontend)"));

    expect(onChange.mock.calls[0][0]).toBe("frontend");
    expect(errors.filter((line) => line.includes("TreeNode"))).toEqual([]);
    spy.mockRestore();
  });
});
