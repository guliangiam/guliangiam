import { useRef, useCallback, useState, memo } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Table } from "antd";
import type { ColumnsType } from "antd/es/table";

type VirtualTableProps<T extends object> = {
  dataSource: T[];
  columns: ColumnsType<T>;
  rowKey: keyof T | ((record: T) => string);
  rowHeight?: number;
  overscan?: number;
  maxHeight?: number;
  onRowMouseEnter?: (record: T) => void;
};

const DEFAULT_ROW_HEIGHT = 54;
const DEFAULT_OVERSCAN = 5;
const DEFAULT_MAX_HEIGHT = 600;

export const VirtualTable = memo(function VirtualTable<T extends object>({
  dataSource,
  columns,
  rowKey,
  rowHeight = DEFAULT_ROW_HEIGHT,
  overscan = DEFAULT_OVERSCAN,
  maxHeight = DEFAULT_MAX_HEIGHT,
  onRowMouseEnter,
}: VirtualTableProps<T>) {
  const parentRef = useRef<HTMLDivElement>(null);
  /* WCAG 2.1.1 Keyboard — track focused row for arrow-key navigation (C-06) */
  const [focusedIndex, setFocusedIndex] = useState(-1);

  const virtualizer = useVirtualizer({
    count: dataSource.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => rowHeight,
    overscan,
  });

  const resolveKey = useCallback(
    (record: T, index: number) => {
      if (typeof rowKey === "function") return rowKey(record);
      return String(record[rowKey] ?? index);
    },
    [rowKey],
  );

  const handleRowKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setFocusedIndex((prev) => Math.min(prev + 1, dataSource.length - 1));
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setFocusedIndex((prev) => Math.max(prev - 1, 0));
      }
    },
    [dataSource.length],
  );

  return (
    <div ref={parentRef} style={{ maxHeight, overflow: "auto" }}>
      <div
        style={{
          height: `${virtualizer.getTotalSize()}px`,
          width: "100%",
          position: "relative",
        }}
      >
        {virtualizer.getVirtualItems().map((virtualRow) => {
          const record = dataSource[virtualRow.index];
          const isFocused = virtualRow.index === focusedIndex;
          return (
            <div
              key={virtualRow.key}
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100%",
                height: `${virtualRow.size}px`,
                transform: `translateY(${virtualRow.start}px)`,
              }}
              /* WCAG 2.1.1 Keyboard — make row focusable with arrow-key nav (C-06) */
              role="row"
              tabIndex={isFocused ? 0 : -1}
              aria-rowindex={virtualRow.index + 1}
              onFocus={() => setFocusedIndex(virtualRow.index)}
              onKeyDown={handleRowKeyDown}
              onMouseEnter={() => {
                onRowMouseEnter?.(record);
                setFocusedIndex(virtualRow.index);
              }}
            >
              <Table<T>
                rowKey={() => resolveKey(record, virtualRow.index)}
                dataSource={[record]}
                columns={columns}
                pagination={false}
                showHeader={virtualRow.index === 0}
                size="middle"
              />
            </div>
          );
        })}
      </div>
    </div>
  );
}) as <T extends object>(props: VirtualTableProps<T>) => React.JSX.Element;
