function Button({ label }: { label: string }) {
    return <button style={{ padding: 10, background: 'blue', color: 'white' }}>{label}</button>
}

export default function CardWithButton() {
    return (
        <div style={{ padding: 20 }}>
            <h1>Static Header</h1>
            <Button label="Click me" />
        </div>
    )
}
