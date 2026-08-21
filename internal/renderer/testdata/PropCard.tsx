interface Props {
    title: string
    count: number
    onClick?: () => void
}

export default function PropCard({ title, count }: Props) {
    return (
        <div style={{ padding: 20 }}>
            <h1>{title}</h1>
            <p>{count} items</p>
        </div>
    )
}
