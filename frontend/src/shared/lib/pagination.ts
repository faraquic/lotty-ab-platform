export function totalPages(total: number, limit: number): number {
  if (limit <= 0 || total <= 0) {
    return 1;
  }
  return Math.max(1, Math.ceil(total / limit));
}

export function lastPageIndex(total: number, limit: number): number {
  return Math.max(0, totalPages(total, limit) - 1);
}

export function offsetForPage(page: number, limit: number): number {
  return Math.max(0, page) * limit;
}
