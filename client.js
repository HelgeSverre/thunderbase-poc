export class ThunderBase {
  constructor(url) {
    this.baseUrl = url;
    this.socket = null;
    this.collections = {};
  }

  collection(name) {
    if (!this.collections[name]) {
      this.collections[name] = new Collection(this, name);
    }
    return this.collections[name];
  }

  connect() {
    return new Promise((resolve, reject) => {
      this.socket = new WebSocket(`${this.baseUrl}/ws`);
      this.socket.onopen = () => resolve();
      this.socket.onerror = (error) => reject(error);
    });
  }
}

class Collection {
  constructor(tb, name) {
    this.tb = tb;
    this.name = name;
    this.listeners = new Map();
  }

  async all(filter = {}) {
    const queryParams = new URLSearchParams(filter).toString();
    const response = await fetch(
      `${this.tb.baseUrl}/${this.name}?${queryParams}`,
    );
    return response.json();
  }

  async getOne(id) {
    const response = await fetch(`${this.tb.baseUrl}/${this.name}/${id}`);
    return response.json();
  }

  async create(data) {
    const response = await fetch(`${this.tb.baseUrl}/${this.name}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    return response.json();
  }

  async update(id, data) {
    const response = await fetch(`${this.tb.baseUrl}/${this.name}/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    return response.json();
  }

  async delete(id) {
    const response = await fetch(`${this.tb.baseUrl}/${this.name}/${id}`, {
      method: "DELETE",
    });
    return response.json();
  }

  subscribe(event, callback) {
    if (!this.tb.socket) {
      throw new Error(
        "WebSocket connection not established. Call connect() first.",
      );
    }

    const listener = (messageEvent) => {
      const data = JSON.parse(messageEvent.data);
      if (
        data.collection === this.name &&
        (event === "*" || data.event === event)
      ) {
        callback(data);
      }
    };

    this.tb.socket.addEventListener("message", listener);
    this.listeners.set(callback, listener);
  }

  unsubscribe(callback) {
    const listener = this.listeners.get(callback);
    if (listener) {
      this.tb.socket.removeEventListener("message", listener);
      this.listeners.delete(callback);
    }
  }
}
