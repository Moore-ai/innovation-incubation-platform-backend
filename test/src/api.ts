// 模拟前端的 API 调用工具
// 使用 tsx 直接运行: npx tsx src/example.ts

const BASE = "http://localhost:8080/api/v1";

interface ApiResponse<T = any> {
  code: number;
  message: string;
  data?: T;
}

export class ApiClient {
  private token: string = "";

  constructor(private base: string = BASE) {}

  setToken(token: string) {
    this.token = token;
  }

  get tokenStr() {
    return this.token;
  }

  private headers(): Record<string, string> {
    const h: Record<string, string> = { "Content-Type": "application/json" };
    if (this.token) h["Authorization"] = `Bearer ${this.token}`;
    return h;
  }

  async post<T = any>(path: string, body?: unknown): Promise<ApiResponse<T>> {
    const r = await fetch(this.base + path, {
      method: "POST",
      headers: this.headers(),
      body: body ? JSON.stringify(body) : undefined,
    });
    return r.json();
  }

  async put<T = any>(path: string, body?: unknown): Promise<ApiResponse<T>> {
    const r = await fetch(this.base + path, {
      method: "PUT",
      headers: this.headers(),
      body: body ? JSON.stringify(body) : undefined,
    });
    return r.json();
  }

  async uploadFile<T = any>(path: string, file: Blob, filename: string): Promise<ApiResponse<T>> {
    const fd = new FormData();
    fd.append("file", file, filename);
    const r = await fetch(this.base + path, {
      method: "POST",
      headers: this.token ? { Authorization: `Bearer ${this.token}` } : {},
      body: fd,
    });
    return r.json();
  }

  /** GET 原始响应（非 JSON，用于文件下载） */
  async getRaw(path: string): Promise<Response> {
    const url = new URL(this.base + path, "http://localhost");
    return fetch(url.href, { method: "GET", headers: this.headers() });
  }

  async del<T = any>(path: string): Promise<ApiResponse<T>> {
    const r = await fetch(this.base + path, {
      method: "DELETE",
      headers: this.headers(),
    });
    return r.json();
  }

  async patch<T = any>(path: string, body?: unknown): Promise<ApiResponse<T>> {
    const r = await fetch(this.base + path, {
      method: "PATCH",
      headers: this.headers(),
      body: body ? JSON.stringify(body) : undefined,
    });
    return r.json();
  }

  async get<T = any>(path: string, params?: Record<string, string>): Promise<ApiResponse<T>> {
    const url = new URL(this.base + path, "http://localhost");
    if (params) for (const [k, v] of Object.entries(params)) url.searchParams.set(k, v);
    const r = await fetch(url.href, {
      method: "GET",
      headers: this.headers(),
    });
    return r.json();
  }
}
