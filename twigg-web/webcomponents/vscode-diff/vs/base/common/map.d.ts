export declare class SetMap<K, V> {
    private map;
    add(key: K, value: V): void;
    forEach(key: K, fn: (value: V) => void): void;
}
