// Simple queue implementation for O(1) push/pop

export class Entry<T> {
    value: T;
    next: Entry<T> | null = null;
    constructor(value: T) { this.value = value; }
}

export class Fifo<T> {
    private head: Entry<T> | null = null;
    private tail: Entry<T> | null = null;

    // O(1)
    push(value: T) {
        const node = new Entry(value);
        if (!this.tail) {
            this.head = this.tail = node;
        } else {
            this.tail.next = node;
            this.tail = node;
        }
    }

    // O(1)
    pop(): T  {
        if (!this.head){
            throw "popped empty queue"
        };
        const val = this.head.value;
        this.head = this.head.next;
        if (!this.head) this.tail = null;
        return val;
    }

    isEmpty() {
        return this.head === null;
    }

    // pops all entries and returns a slice with them
    drain(): T[] {
        const out: T[] = [];
        while (this.head) out.push(this.pop()!);
        return out;
    }
}