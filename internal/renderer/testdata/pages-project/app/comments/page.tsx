import { db, comments } from "@/db"

export default async function CommentsPage() {
  const rows = await db.select().from(comments).where(true)

  return (
    <main>
      <h1>Comments</h1>
      <p>{rows.length}</p>
    </main>
  )
}
