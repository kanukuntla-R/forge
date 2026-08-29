import { prisma } from "../../services/db-client"

export default async function BrokenPage() {
  const users = await prisma.user.findMany()

  return (
    <main>
      <h1>Broken</h1>
      <p>{users.length}</p>
    </main>
  )
}
