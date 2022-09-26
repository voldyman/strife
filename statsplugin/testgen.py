from random import random


def day_distribution():
    counts = []
    for _ in range(24):
        counts.append(int(random() * 10))
    return counts


def gencode(days):
    totals = []
    print('distribution := [][]int{')
    for _ in range(days):
        counts = day_distribution()
        total = sum(counts)
        totals.append(total)
        counts_str = ', '.join(map(lambda x: str(x), counts))
        print(f"{{{counts_str}}}, // {total}")
    print('}')

    print()
    print()
    for i in range(len(totals)):
        print(
            f'assert.Equal(t, sumDay(weekMatrix[{i}]), {totals[i]}, "day {i} does not match expected total")')


if __name__ == '__main__':
    gencode(7)
