import { useEffect, useState } from 'react';

function fmt(d: Date): string {
    return d.toISOString().split('T')[1].split('.')[0];
}

export default function SystemClock() {
    const [time, setTime] = useState<string>(() => fmt(new Date()));

    useEffect(() => {
        const id = setInterval(() => setTime(fmt(new Date())), 1000);
        return () => clearInterval(id);
    }, []);

    return <span id="clock">{time}</span>;
}
