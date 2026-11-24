/** Go presenter.ListResponse 的前端镜像：{ items, page: { page, pageSize, total } }。 */
export interface PageMeta {
  page: number;
  pageSize: number;
  total: number;
}

export interface Paginated<T> {
  items: T[];
  page: PageMeta;
}
