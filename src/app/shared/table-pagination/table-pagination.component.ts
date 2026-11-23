import { Component, computed, effect, Input, signal, Signal } from '@angular/core';

export class TablePagination<T extends object> {
  readonly page = signal(1);
  private readonly selectedPageSize = signal(25);
  readonly search = signal('');
  readonly count = computed(() => this.items().length);
  readonly pageSize = computed(() => this.count() > 25 ? this.selectedPageSize() : 25);
  readonly filtered = computed(() => {
    const query = this.alwaysSearch || this.count() > 25 ? this.search().trim().toLocaleLowerCase() : '';
    return query
      ? this.items().filter(item => this.searchValues(item).some(value => value.toLocaleLowerCase().includes(query)))
      : this.items();
  });
  readonly filteredCount = computed(() => this.filtered().length);
  readonly totalPages = computed(() => Math.max(1, Math.ceil(this.filtered().length / this.pageSize())));
  readonly currentPage = computed(() => Math.min(this.page(), this.totalPages()));
  readonly rows = computed(() => {
    const offset = (this.currentPage() - 1) * this.pageSize();
    return this.filtered().slice(offset, offset + this.pageSize());
  });

  constructor(
    private readonly items: Signal<T[]>,
    private readonly searchValues: (item: T) => string[] = item => Object.values(item).map(String),
    readonly alwaysSearch = false
  ) {
    effect(() => {
      if (!this.alwaysSearch && this.count() <= 25 && this.search()) {
        this.search.set('');
      }
      if (this.page() > this.totalPages()) {
        this.page.set(this.totalPages());
      }
    });
  }

  setSearch(value: string): void {
    this.search.set(value);
    this.page.set(1);
  }

  setPageSize(value: number): void {
    if ([5, 10, 25].includes(value)) {
      this.selectedPageSize.set(value);
      this.page.set(1);
    }
  }

  setPage(value: number): void {
    this.page.set(Math.max(1, Math.min(value, this.totalPages())));
  }

  reset(): void {
    this.search.set('');
    this.page.set(1);
  }
}

interface PaginationControls {
  alwaysSearch: boolean;
  count: Signal<number>;
  filteredCount: Signal<number>;
  search: Signal<string>;
  pageSize: Signal<number>;
  currentPage: Signal<number>;
  totalPages: Signal<number>;
  setSearch(value: string): void;
  setPageSize(value: number): void;
  setPage(value: number): void;
}

@Component({
  selector: 'app-table-pagination',
  templateUrl: './table-pagination.component.html',
  styleUrl: './table-pagination.component.scss'
})
export class TablePaginationComponent {
  @Input({ required: true }) pagination!: PaginationControls;
  @Input({ required: true }) label = '';
  @Input() placement: 'top' | 'bottom' = 'bottom';

  changeSearch(event: Event): void {
    this.pagination.setSearch((event.target as HTMLInputElement).value);
  }

  changePageSize(event: Event): void {
    this.pagination.setPageSize(Number((event.target as HTMLSelectElement).value));
  }
}
