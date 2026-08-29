import { prisma } from "@/lib/prisma"

export default async function PostDetailPage({
  params,
}: {
  params: { id: string }
}) {
  const post = await prisma.post.findUnique({ where: { id: parseInt(params.id) } })

  return (
    <main>
      <h1>{post?.title}</h1>
      <p>id param: {params.id}</p>
    </main>
  )
}
